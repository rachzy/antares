"""The Fang servicer's translation between proto and library calls."""

from __future__ import annotations

from pathlib import Path

import grpc
import pytest
from antares.fang.v1 import fang_pb2
from fang.errors import DataValidationError, SchemaVersionError
from fang.service import RunInfo

from fang_service import servicer as servicer_module
from fang_service.convert import from_struct, to_struct
from fang_service.servicer import FangServicer


class _Context:
    """Records the status a handler aborts with."""

    def __init__(self) -> None:
        self.code: grpc.StatusCode | None = None
        self.details: str = ""

    def abort(self, code, details):
        self.code = code
        self.details = details
        raise _Aborted(details)


class _Aborted(Exception):
    pass


def test_struct_roundtrip_preserves_mixed_types():
    original = {"period_days": 3.5, "detection_status": "accepted", "flagged": True}
    assert from_struct(to_struct(original)) == original


def test_struct_roundtrip_carries_missing_values_as_null():
    """MessageToDict raises on NaN, so a record with a missing feature must not break it."""
    row = {"period_days": 3.5, "vshape_metric": float("nan"), "snr": float("inf")}
    assert from_struct(to_struct(row)) == {
        "period_days": 3.5,
        "vshape_metric": None,
        "snr": None,
    }


def test_list_runs_is_empty_before_any_training(tmp_path):
    service = FangServicer(artifact_dir=tmp_path / "never_trained")
    response = service.ListRuns(fang_pb2.ListRunsRequest(), _Context())
    assert list(response.runs) == []


def test_list_runs_maps_library_fields(tmp_path, monkeypatch):
    from fang.service import RunInfo

    monkeypatch.setattr(
        servicer_module,
        "list_runs",
        lambda directory: [
            RunInfo(
                run_id="20260929T120000Z-abcd1234",
                created_at="2026-09-29T12:00:00Z",
                schema_version=1,
                selected_model="lightgbm",
                threshold=0.42,
                path=Path(directory),
            )
        ],
    )

    service = FangServicer(artifact_dir=tmp_path)
    response = service.ListRuns(fang_pb2.ListRunsRequest(), _Context())

    assert len(response.runs) == 1
    assert response.runs[0].run_id == "20260929T120000Z-abcd1234"
    assert response.runs[0].schema_version == 1
    assert response.runs[0].threshold == pytest.approx(0.42)


def test_predict_returns_rows_and_the_run_id(tmp_path, monkeypatch):
    monkeypatch.setattr(
        servicer_module,
        "predict_features",
        lambda rows, model_dir, star_id: [
            {**rows[0], "prob_stack": 0.9, "model_run_id": "run-1"}
        ],
    )
    monkeypatch.setattr(
        servicer_module,
        "_resolve_run",
        lambda base, run_id: RunInfo(
            run_id="run-1",
            created_at="",
            schema_version=1,
            selected_model="lightgbm",
            threshold=0.5,
            path=tmp_path / "run-1",
        ),
    )

    service = FangServicer(artifact_dir=tmp_path)
    request = fang_pb2.PredictRequest(
        features=[to_struct({"period_days": 3.5})],
        star_id="KIC-8120608",
    )
    response = service.Predict(request, _Context())

    assert response.model_run_id == "run-1"
    assert response.schema_version == 1
    assert from_struct(response.predictions[0])["prob_stack"] == pytest.approx(0.9)


def test_predict_reports_the_served_runs_schema_version(tmp_path, monkeypatch):
    """The response must carry the run's own schema version, not a hard-coded 1."""
    monkeypatch.setattr(
        servicer_module,
        "predict_features",
        lambda rows, model_dir, star_id: [{**rows[0], "model_run_id": "run-2"}],
    )
    monkeypatch.setattr(
        servicer_module,
        "_resolve_run",
        lambda base, run_id: RunInfo(
            run_id="run-2",
            created_at="",
            schema_version=2,
            selected_model="lightgbm",
            threshold=0.5,
            path=tmp_path / "run-2",
        ),
    )

    service = FangServicer(artifact_dir=tmp_path)
    request = fang_pb2.PredictRequest(features=[to_struct({"period_days": 3.5})])
    response = service.Predict(request, _Context())

    assert response.schema_version == 2
    assert response.model_run_id == "run-2"


def test_predict_with_a_model_that_mismatches_the_schema_is_a_server_fault(
    tmp_path, monkeypatch
):
    """SchemaVersionError comes from load_model: the deployment is wrong, not the caller."""

    def mismatched(rows, model_dir, star_id):
        raise SchemaVersionError("model expects schema version 2, installed is 1")

    monkeypatch.setattr(servicer_module, "predict_features", mismatched)
    monkeypatch.setattr(
        servicer_module,
        "_resolve_run",
        lambda base, run_id: RunInfo(
            run_id="run-1",
            created_at="",
            schema_version=1,
            selected_model="lightgbm",
            threshold=0.5,
            path=tmp_path / "run-1",
        ),
    )

    context = _Context()
    request = fang_pb2.PredictRequest(features=[to_struct({"period_days": 3.5})])

    with pytest.raises(_Aborted):
        FangServicer(artifact_dir=tmp_path).Predict(request, context)

    assert context.code == grpc.StatusCode.FAILED_PRECONDITION


def test_predict_with_unknown_run_id_aborts_not_found(tmp_path):
    service = FangServicer(artifact_dir=tmp_path)
    context = _Context()
    request = fang_pb2.PredictRequest(
        features=[to_struct({"period_days": 3.5})],
        run_id="20990101T000000Z-ffffffff",
    )

    with pytest.raises(_Aborted):
        service.Predict(request, context)

    assert context.code == grpc.StatusCode.NOT_FOUND
    assert "20990101T000000Z-ffffffff" in context.details


def test_predict_run_id_cannot_traverse_out_of_the_artifact_dir(tmp_path):
    """run_id selects a pickle to load; a traversal must be NOT_FOUND, not a path."""
    outside = tmp_path / "outside"
    outside.mkdir()
    (outside / "model.joblib").write_bytes(b"not for you")

    artifacts = tmp_path / "models"
    artifacts.mkdir()
    service = FangServicer(artifact_dir=artifacts)
    context = _Context()
    request = fang_pb2.PredictRequest(
        features=[to_struct({"period_days": 3.5})],
        run_id="../outside",
    )

    with pytest.raises(_Aborted):
        service.Predict(request, context)

    assert context.code == grpc.StatusCode.NOT_FOUND


def test_predict_with_a_missing_column_aborts_invalid_argument(tmp_path, monkeypatch):
    def drifted(rows, model_dir, star_id):
        raise DataValidationError("missing required feature columns ['period_days']")

    monkeypatch.setattr(servicer_module, "predict_features", drifted)
    monkeypatch.setattr(
        servicer_module,
        "_resolve_run",
        lambda base, run_id: RunInfo(
            run_id="run-1",
            created_at="",
            schema_version=1,
            selected_model="lightgbm",
            threshold=0.5,
            path=tmp_path / "run-1",
        ),
    )

    service = FangServicer(artifact_dir=tmp_path)
    context = _Context()
    request = fang_pb2.PredictRequest(features=[to_struct({"MES": 12.0})])

    with pytest.raises(_Aborted):
        service.Predict(request, context)

    assert context.code == grpc.StatusCode.INVALID_ARGUMENT
    assert "period_days" in context.details


def test_predict_with_no_features_aborts_invalid_argument(tmp_path):
    service = FangServicer(artifact_dir=tmp_path)
    context = _Context()

    with pytest.raises(_Aborted):
        service.Predict(fang_pb2.PredictRequest(), context)

    assert context.code == grpc.StatusCode.INVALID_ARGUMENT
