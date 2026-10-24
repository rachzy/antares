"""The Fang servicer's translation between proto and library calls."""

from __future__ import annotations

import json
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
        lambda rows, model_dir, star_id: [{**rows[0], "prob_stack": 0.9, "model_run_id": "run-1"}],
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


def test_predict_with_a_model_that_mismatches_the_schema_is_a_server_fault(tmp_path, monkeypatch):
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


def _run(tmp_path, run_id="run-1"):
    directory = tmp_path / run_id
    directory.mkdir()
    return RunInfo(
        run_id=run_id,
        created_at="",
        schema_version=1,
        selected_model="lightgbm",
        threshold=0.42,
        path=directory,
    )


def test_get_run_settings_returns_the_runs_best_params(tmp_path, monkeypatch):
    run = _run(tmp_path)
    (run.path / "summary.json").write_text(
        '{"best_params": {"lightgbm": {"n_estimators": 300, "learning_rate": 0.05}, '
        '"meta": {"C": 10}}}'
    )
    monkeypatch.setattr(servicer_module, "_resolve_run", lambda base, run_id: run)

    response = FangServicer(artifact_dir=tmp_path).GetRunSettings(
        fang_pb2.GetRunSettingsRequest(run_id="run-1"), _Context()
    )

    assert response.run_id == "run-1"
    assert response.selected_model == "lightgbm"
    assert response.threshold == pytest.approx(0.42)
    assert from_struct(response.best_params) == {
        "lightgbm": {"n_estimators": 300, "learning_rate": 0.05},
        "meta": {"C": 10},
    }


def test_get_run_settings_is_not_found_for_an_unservable_run(tmp_path):
    context = _Context()
    with pytest.raises(_Aborted):
        FangServicer(artifact_dir=tmp_path).GetRunSettings(
            fang_pb2.GetRunSettingsRequest(run_id="../x"), context
        )
    assert context.code == grpc.StatusCode.NOT_FOUND


@pytest.mark.parametrize("summary", [None, "not json", "[]", '{"best_params": 3}'])
def test_get_run_settings_fails_precondition_without_usable_params(tmp_path, monkeypatch, summary):
    run = _run(tmp_path)
    if summary is not None:
        (run.path / "summary.json").write_text(summary)
    monkeypatch.setattr(servicer_module, "_resolve_run", lambda base, run_id: run)

    context = _Context()
    with pytest.raises(_Aborted):
        FangServicer(artifact_dir=tmp_path).GetRunSettings(
            fang_pb2.GetRunSettingsRequest(), context
        )
    assert context.code == grpc.StatusCode.FAILED_PRECONDITION


_REPORT = {
    "dataset": {
        "n_rows": 120,
        "n_stars": 40,
        "n_files": 41,
        "n_features": 36,
        "n_unlabelled": 3,
        "n_positive": 30,
        "n_negative": 90,
        "status_counts": {"accepted": 100, "provisional": 20},
        "stars": ["KIC 1", "KIC 2"],
    },
    "models": {
        "lightgbm": {
            "crossfit_metrics": {"confusion_matrix_counts": {"tp": 28, "fp": 6, "fn": 2, "tn": 84}}
        },
        "stack": {
            "crossfit_metrics": {"confusion_matrix_counts": {"tp": 29, "fp": 7, "fn": 1, "tn": 83}}
        },
    },
}


def _dataset_run(tmp_path, monkeypatch, report=_REPORT, metrics=None):
    run = _run(tmp_path)
    if report is not None:
        (run.path / "training_report.json").write_text(json.dumps(report))
    if metrics is not None:
        (run.path / "evaluation").mkdir()
        (run.path / "evaluation" / "metrics.json").write_text(json.dumps(metrics))
    monkeypatch.setattr(servicer_module, "_resolve_run", lambda base, run_id: run)
    return run


def _get_dataset(tmp_path, context=None):
    return FangServicer(artifact_dir=tmp_path).GetRunDataset(
        fang_pb2.GetRunDatasetRequest(), context or _Context()
    )


def _counts(matrix):
    return (matrix.tp, matrix.fp, matrix.fn, matrix.tn, matrix.source)


def test_get_run_dataset_reports_counts_and_cross_fit_matrices(tmp_path, monkeypatch):
    _dataset_run(tmp_path, monkeypatch)

    response = _get_dataset(tmp_path)

    assert response.run_id == "run-1"
    assert response.selected_model == "lightgbm"
    assert (response.n_rows, response.n_stars, response.n_files) == (120, 40, 41)
    assert (response.n_features, response.n_unlabelled) == (36, 3)
    assert (response.n_positive, response.n_negative) == (30, 90)
    assert dict(response.status_counts) == {"accepted": 100, "provisional": 20}
    assert set(response.confusion_matrices) == {"lightgbm", "stack"}
    assert _counts(response.confusion_matrices["lightgbm"]) == (28, 6, 2, 84, "cross_fit")
    assert _counts(response.confusion_matrices["stack"]) == (29, 7, 1, 83, "cross_fit")


def test_get_run_dataset_prefers_held_out_matrices_per_model(tmp_path, monkeypatch):
    metrics = {
        "pooled_metrics": {
            "lightgbm": {"confusion_matrix_counts": {"tp": 25, "fp": 9, "fn": 5, "tn": 81}}
        }
    }
    _dataset_run(tmp_path, monkeypatch, metrics=metrics)

    matrices = _get_dataset(tmp_path).confusion_matrices

    assert _counts(matrices["lightgbm"]) == (25, 9, 5, 81, "held_out")
    assert _counts(matrices["stack"]) == (29, 7, 1, 83, "cross_fit")


def test_get_run_dataset_has_no_matrices_when_none_are_recorded(tmp_path, monkeypatch):
    report = {"dataset": _REPORT["dataset"], "models": {}}
    _dataset_run(tmp_path, monkeypatch, report=report)

    response = _get_dataset(tmp_path)

    assert dict(response.confusion_matrices) == {}
    assert response.n_rows == 120


def test_get_run_dataset_is_not_found_for_an_unservable_run(tmp_path):
    context = _Context()
    with pytest.raises(_Aborted):
        FangServicer(artifact_dir=tmp_path).GetRunDataset(
            fang_pb2.GetRunDatasetRequest(run_id="../x"), context
        )
    assert context.code == grpc.StatusCode.NOT_FOUND


@pytest.mark.parametrize("report", [None, "not json", "[]", '{"dataset": 3}'])
def test_get_run_dataset_fails_precondition_without_a_usable_report(tmp_path, monkeypatch, report):
    run = _dataset_run(tmp_path, monkeypatch, report=None)
    if report is not None:
        (run.path / "training_report.json").write_text(report)

    context = _Context()
    with pytest.raises(_Aborted):
        _get_dataset(tmp_path, context)
    assert context.code == grpc.StatusCode.FAILED_PRECONDITION
