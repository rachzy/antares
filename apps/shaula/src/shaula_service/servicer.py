"""Translation between the Shaula proto contract and the Shaula library."""

from __future__ import annotations

import queue
import sys
import threading
from collections.abc import Callable

import grpc
from antares.shaula.v1 import shaula_pb2, shaula_pb2_grpc
from shaula import __version__ as library_version
from shaula.api import ProgressEvent, TargetNotFound, extract, resolve

from .convert import to_struct

SERVICE_VERSION = "0.0.1a0"


class _Cancelled(Exception):
    """Raised from the progress callback to stop an extraction nobody is reading."""


class _StdoutRouter:
    """Tees the library's print() output, per thread, to a line callback.

    The library narrates most of its work on stdout and emits a ProgressEvent only
    at stage boundaries, so forwarding events alone drops nearly every console line.
    """

    _lock = threading.Lock()
    _users = 0
    _original = None
    _local = threading.local()

    def __init__(self, original) -> None:
        self._wrapped = original

    def write(self, text: str) -> int:
        sink = getattr(self._local, "sink", None)
        if sink is not None:
            sink(text)
        return self._wrapped.write(text)

    def __getattr__(self, name: str):
        return getattr(self._wrapped, name)

    @classmethod
    def attach(cls, on_line: Callable[[str], None]) -> Callable[[], None]:
        """Route this thread's stdout lines to on_line; the returned function detaches."""
        buffer = ""

        def sink(text: str) -> None:
            nonlocal buffer
            buffer += text
            *lines, buffer = buffer.split("\n")
            for line in lines:
                if line.strip():
                    on_line(line.rstrip())

        with cls._lock:
            if cls._users == 0:
                cls._original = sys.stdout
                sys.stdout = cls(sys.stdout)
            cls._users += 1
        cls._local.sink = sink

        def detach() -> None:
            cls._local.sink = None
            with cls._lock:
                cls._users -= 1
                if cls._users == 0:
                    sys.stdout = cls._original
            if buffer.strip():
                on_line(buffer.rstrip())

        return detach


def _status_for(error: Exception) -> tuple[grpc.StatusCode, str]:
    """ValueError and RuntimeError are the library's own data and config failures."""
    if isinstance(error, OSError):
        return grpc.StatusCode.UNAVAILABLE, str(error) or type(error).__name__
    if isinstance(error, (ValueError, RuntimeError)):
        return grpc.StatusCode.FAILED_PRECONDITION, str(error)
    return grpc.StatusCode.INTERNAL, f"{type(error).__name__}: {error}"


class ShaulaServicer(shaula_pb2_grpc.ShaulaServicer):
    def GetVersion(self, request, context):
        return shaula_pb2.GetVersionResponse(
            library_version=library_version,
            service_version=SERVICE_VERSION,
        )

    def ResolveTarget(self, request, context):
        ra_deg = request.ra_deg if request.HasField("ra_deg") else None
        dec_deg = request.dec_deg if request.HasField("dec_deg") else None
        try:
            resolved = resolve(
                request.query or None,
                request.mission or "Kepler",
                ra_deg=ra_deg,
                dec_deg=dec_deg,
            )
        except TargetNotFound as error:
            context.abort(grpc.StatusCode.NOT_FOUND, str(error))
        except ValueError as error:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))

        return shaula_pb2.ResolveTargetResponse(
            catalog=resolved.catalog,
            catalog_id=resolved.catalog_id,
            ra_deg=resolved.ra_deg,
            dec_deg=resolved.dec_deg,
            display_name=resolved.display_name,
            designation=resolved.designation,
        )

    def ExtractFeatures(self, request, context):
        events: queue.Queue = queue.Queue()
        outcome: dict[str, object] = {}
        cancelled = threading.Event()
        context.add_callback(cancelled.set)
        stage = "downloading"

        def on_progress(event: ProgressEvent) -> None:
            nonlocal stage
            if cancelled.is_set():
                raise _Cancelled
            stage = event.stage
            events.put(event)

        def on_line(line: str) -> None:
            if cancelled.is_set():
                raise _Cancelled
            events.put(ProgressEvent(stage=stage, message=line))

        def run() -> None:
            detach = _StdoutRouter.attach(on_line)
            try:
                outcome["result"] = extract(
                    request.target,
                    request.mission or "Kepler",
                    sigma_clip=request.sigma_clip or 5.0,
                    download_all=request.download_all,
                    author=request.author or None,
                    exptime=request.exptime or None,
                    use_tls=request.use_tls,
                    mask_eclipses=request.mask_eclipses,
                    progress=on_progress,
                )
            except Exception as error:  # noqa: BLE001 - surfaced to the client below
                outcome["error"] = error
            finally:
                try:
                    detach()
                except _Cancelled:
                    pass
                events.put(None)

        worker = threading.Thread(target=run, daemon=True)
        worker.start()

        try:
            while True:
                event = events.get()
                if event is None:
                    break
                yield shaula_pb2.ExtractFeaturesEvent(progress=self._to_progress(event))
        finally:
            cancelled.set()

        worker.join()

        error = outcome.get("error")
        if error is not None:
            context.abort(*_status_for(error))

        result = outcome["result"]
        yield shaula_pb2.ExtractFeaturesEvent(
            result=shaula_pb2.FeatureSet(
                target=result.target,
                mission=result.mission,
                features=[to_struct(row) for row in result.features],
                library_version=result.shaula_version,
            )
        )

    @staticmethod
    def _to_progress(event: ProgressEvent) -> shaula_pb2.Progress:
        return shaula_pb2.Progress(
            stage=event.stage,
            message=event.message,
            has_fraction=event.fraction is not None,
            fraction=event.fraction or 0.0,
        )
