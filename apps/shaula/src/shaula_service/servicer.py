"""Translation between the Shaula proto contract and the Shaula library."""

from __future__ import annotations

import queue
import threading

import grpc
from antares.shaula.v1 import shaula_pb2, shaula_pb2_grpc
from shaula import __version__ as library_version
from shaula.api import ProgressEvent, TargetNotFound, extract, resolve

from .convert import to_struct

SERVICE_VERSION = "0.0.1a0"


class _Cancelled(Exception):
    """Raised from the progress callback to stop an extraction nobody is reading."""


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
        try:
            resolved = resolve(request.query, request.mission or "Kepler")
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

        def on_progress(event: ProgressEvent) -> None:
            if cancelled.is_set():
                raise _Cancelled
            events.put(event)

        def run() -> None:
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
