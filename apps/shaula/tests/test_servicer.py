"""The Shaula servicer's translation between proto and library calls."""

from __future__ import annotations

import threading

import grpc
import pytest
from antares.shaula.v1 import shaula_pb2
from shaula.api import (
    ExtractionResult,
    ProgressEvent,
    ResolvedTarget,
    TargetNotFound,
)

from shaula_service import servicer as servicer_module
from shaula_service.convert import from_struct
from shaula_service.servicer import ShaulaServicer


class _Context:
    def __init__(self) -> None:
        self.code: grpc.StatusCode | None = None
        self.details: str = ""
        self.callbacks: list = []

    def add_callback(self, callback):
        self.callbacks.append(callback)
        return True

    def abort(self, code, details):
        self.code = code
        self.details = details
        raise _Aborted(details)


class _Aborted(Exception):
    pass


def test_get_version_reports_the_library_version():
    response = ShaulaServicer().GetVersion(shaula_pb2.GetVersionRequest(), _Context())
    assert response.library_version
    assert response.service_version


def test_resolve_target_maps_every_field(monkeypatch):
    monkeypatch.setattr(
        servicer_module,
        "resolve",
        lambda query, mission: ResolvedTarget(
            catalog="KIC",
            catalog_id="8120608",
            ra_deg=285.6794,
            dec_deg=43.9174,
            display_name="Kepler-186",
        ),
    )

    request = shaula_pb2.ResolveTargetRequest(query="Kepler-186f", mission="Kepler")
    response = ShaulaServicer().ResolveTarget(request, _Context())

    assert response.catalog == "KIC"
    assert response.catalog_id == "8120608"
    assert response.ra_deg == pytest.approx(285.6794)
    assert response.dec_deg == pytest.approx(43.9174)
    assert response.display_name == "Kepler-186"
    assert response.designation == "KIC 8120608"


def test_unresolvable_target_aborts_not_found(monkeypatch):
    def missing(query, mission):
        raise TargetNotFound(f"No KIC entry found for {query!r}.")

    monkeypatch.setattr(servicer_module, "resolve", missing)

    context = _Context()
    request = shaula_pb2.ResolveTargetRequest(query="NotAStar-999", mission="Kepler")

    with pytest.raises(_Aborted):
        ShaulaServicer().ResolveTarget(request, context)

    assert context.code == grpc.StatusCode.NOT_FOUND
    assert "NotAStar-999" in context.details


def test_unsupported_mission_aborts_invalid_argument(monkeypatch):
    def unsupported(query, mission):
        raise ValueError(f"Unsupported mission {mission!r}")

    monkeypatch.setattr(servicer_module, "resolve", unsupported)

    context = _Context()
    request = shaula_pb2.ResolveTargetRequest(query="Kepler-186", mission="Hubble")

    with pytest.raises(_Aborted):
        ShaulaServicer().ResolveTarget(request, context)

    assert context.code == grpc.StatusCode.INVALID_ARGUMENT
    assert "Hubble" in context.details


def _fake_extract(events, result=None, failure=None):
    """Build a stand-in for shaula.api.extract that replays `events`."""

    def extract(target, mission, *, progress=None, **kwargs):
        for event in events:
            if progress is not None:
                progress(event)
        if failure is not None:
            raise failure
        return result

    return extract


def test_a_feature_row_with_a_missing_value_survives_the_struct(monkeypatch):
    """Real rows hold NaN (planet_radius_* when the light curve has no stellar radius).

    MessageToDict raises on NaN, so a FeatureSet built from such a row must carry
    the missing value as null rather than fail the whole extraction at the very end.
    """
    monkeypatch.setattr(
        servicer_module,
        "extract",
        _fake_extract(
            events=[],
            result=ExtractionResult(
                target="Kepler-11",
                mission="Kepler",
                features=[{"period_days": 3.5, "planet_radius_rearth": float("nan")}],
                shaula_version="0.0.1a0",
            ),
        ),
    )

    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")
    emitted = list(ShaulaServicer().ExtractFeatures(request, _Context()))

    row = from_struct(emitted[-1].result.features[0])
    assert row["period_days"] == pytest.approx(3.5)
    assert row["planet_radius_rearth"] is None


def test_streams_progress_then_exactly_one_result(monkeypatch):
    monkeypatch.setattr(
        servicer_module,
        "extract",
        _fake_extract(
            events=[
                ProgressEvent(stage="downloading", message="Downloading"),
                ProgressEvent(stage="period_search", message="candidate 0"),
            ],
            result=ExtractionResult(
                target="Kepler-11",
                mission="Kepler",
                features=[{"period_days": 3.5, "MES": 12.0}],
                shaula_version="0.0.1a0",
            ),
        ),
    )

    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")
    emitted = list(ShaulaServicer().ExtractFeatures(request, _Context()))

    progress = [e for e in emitted if e.WhichOneof("event") == "progress"]
    results = [e for e in emitted if e.WhichOneof("event") == "result"]

    assert [p.progress.stage for p in progress] == ["downloading", "period_search"]
    assert len(results) == 1
    assert results[0].result.target == "Kepler-11"
    assert results[0].result.library_version == "0.0.1a0"
    assert from_struct(results[0].result.features[0])["MES"] == pytest.approx(12.0)


def test_fraction_is_flagged_rather_than_defaulted(monkeypatch):
    """A missing fraction must be distinguishable from a fraction of 0.0."""
    monkeypatch.setattr(
        servicer_module,
        "extract",
        _fake_extract(
            events=[
                ProgressEvent(stage="downloading", message="no fraction"),
                ProgressEvent(stage="detrending", message="half", fraction=0.5),
            ],
            result=ExtractionResult(
                target="Kepler-11", mission="Kepler", features=[], shaula_version="0.0.1a0"
            ),
        ),
    )

    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")
    emitted = list(ShaulaServicer().ExtractFeatures(request, _Context()))
    progress = [e.progress for e in emitted if e.WhichOneof("event") == "progress"]

    assert progress[0].has_fraction is False
    assert progress[1].has_fraction is True
    assert progress[1].fraction == pytest.approx(0.5)


def test_download_failure_aborts_after_emitting_progress(monkeypatch):
    monkeypatch.setattr(
        servicer_module,
        "extract",
        _fake_extract(
            events=[ProgressEvent(stage="downloading", message="Downloading")],
            failure=ValueError("No Kepler light curves found for 'Nope'"),
        ),
    )

    context = _Context()
    request = shaula_pb2.ExtractFeaturesRequest(target="Nope", mission="Kepler")

    emitted = []
    with pytest.raises(_Aborted):
        emitted.extend(ShaulaServicer().ExtractFeatures(request, context))

    assert [e.WhichOneof("event") for e in emitted] == ["progress"]
    assert context.code == grpc.StatusCode.FAILED_PRECONDITION
    assert "No Kepler light curves found" in context.details


def test_a_target_with_no_candidates_still_yields_a_result(monkeypatch):
    """Zero candidates is a valid scientific answer, not an error."""
    monkeypatch.setattr(
        servicer_module,
        "extract",
        _fake_extract(
            events=[],
            result=ExtractionResult(
                target="Kepler-11", mission="Kepler", features=[], shaula_version="0.0.1a0"
            ),
        ),
    )

    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")
    emitted = list(ShaulaServicer().ExtractFeatures(request, _Context()))

    assert [e.WhichOneof("event") for e in emitted] == ["result"]
    assert list(emitted[0].result.features) == []


def _failing_extract(error):
    return _fake_extract(events=[], failure=error)


@pytest.mark.parametrize(
    ("error", "code"),
    [
        (ConnectionError("MAST timed out"), grpc.StatusCode.UNAVAILABLE),
        (RuntimeError("use_tls requires the tls extra"), grpc.StatusCode.FAILED_PRECONDITION),
        (KeyError("flux"), grpc.StatusCode.INTERNAL),
    ],
)
def test_extraction_errors_map_to_distinct_statuses(monkeypatch, error, code):
    monkeypatch.setattr(servicer_module, "extract", _failing_extract(error))

    context = _Context()
    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")

    with pytest.raises(_Aborted):
        list(ShaulaServicer().ExtractFeatures(request, context))

    assert context.code == code


def test_an_unexpected_error_names_its_type(monkeypatch):
    monkeypatch.setattr(servicer_module, "extract", _failing_extract(KeyError("flux")))

    context = _Context()
    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")

    with pytest.raises(_Aborted):
        list(ShaulaServicer().ExtractFeatures(request, context))

    assert "KeyError" in context.details


def test_abandoning_the_stream_cancels_the_extraction(monkeypatch):
    """A consumer that stops reading must make the library's progress callback raise."""
    resume = threading.Event()
    finished = threading.Event()
    outcome: dict[str, bool] = {}

    def extract(target, mission, *, progress=None, **kwargs):
        progress(ProgressEvent(stage="downloading", message="first"))
        resume.wait(5)
        try:
            progress(ProgressEvent(stage="period_search", message="second"))
        except Exception:
            outcome["cancelled"] = True
            raise
        finally:
            finished.set()

    monkeypatch.setattr(servicer_module, "extract", extract)

    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")
    stream = ShaulaServicer().ExtractFeatures(request, _Context())
    next(stream)
    stream.close()
    resume.set()

    assert finished.wait(5)
    assert outcome.get("cancelled") is True


def test_a_terminated_rpc_cancels_the_extraction(monkeypatch):
    """gRPC cancel or deadline fires the registered callback while the handler is blocked."""
    resume = threading.Event()
    finished = threading.Event()
    outcome: dict[str, bool] = {}

    def extract(target, mission, *, progress=None, **kwargs):
        progress(ProgressEvent(stage="downloading", message="first"))
        resume.wait(5)
        try:
            progress(ProgressEvent(stage="period_search", message="second"))
        except Exception:
            outcome["cancelled"] = True
            raise
        finally:
            finished.set()

    monkeypatch.setattr(servicer_module, "extract", extract)

    context = _Context()
    request = shaula_pb2.ExtractFeaturesRequest(target="Kepler-11", mission="Kepler")
    stream = ShaulaServicer().ExtractFeatures(request, context)
    next(stream)
    for callback in context.callbacks:
        callback()
    resume.set()

    assert finished.wait(5)
    assert outcome.get("cancelled") is True
