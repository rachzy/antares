"""Translation between the Fang proto contract and the Fang library."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import grpc
from antares.fang.v1 import fang_pb2, fang_pb2_grpc
from fang import __version__ as library_version
from fang.errors import DataValidationError, SchemaVersionError
from fang.service import RunInfo, latest_run, list_runs, predict_features

from .convert import from_struct, to_struct

SERVICE_VERSION = "0.0.1a0"


def _resolve_run(artifact_dir: Path, run_id: str) -> RunInfo:
    """The run to score with, or raise FileNotFoundError.

    ``run_id`` arrives from a network request and selects a pickle that will be
    loaded, so it is matched against the runs ``list_runs`` reports rather than
    joined onto a path: a value like ``../x`` must never select a directory, and
    a run that is not safe to serve must not become selectable by naming it.
    The whole ``RunInfo`` is returned so the response reports the served run's
    own id and schema version instead of values hard-coded here.
    """
    if run_id:
        for run in list_runs(artifact_dir):
            if run.run_id == run_id:
                return run
        raise FileNotFoundError(f"No servable run {run_id!r} under {artifact_dir}.")
    return latest_run(artifact_dir)


def _read_best_params(run: RunInfo) -> dict[str, Any]:
    """The tuned hyperparameters recorded in the run's ``summary.json``.

    Read from the sidecar so the pickled bundle is never loaded on a request path.
    Raises ValueError when the file is missing or does not carry them.
    """
    try:
        summary = json.loads((run.path / "summary.json").read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise ValueError(f"Run {run.run_id!r} has no readable summary.json.") from error
    best_params = summary.get("best_params") if isinstance(summary, dict) else None
    if isinstance(best_params, dict):
        return best_params
    raise ValueError(f"Run {run.run_id!r} records no best_params.")


def _read_json_mapping(path: Path) -> dict[str, Any] | None:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    return payload if isinstance(payload, dict) else None


def _count_matrix(raw: Any, source: str) -> fang_pb2.ConfusionMatrix | None:
    if not isinstance(raw, dict):
        return None
    try:
        counts = {key: int(raw[key]) for key in ("tp", "fp", "fn", "tn")}
    except (KeyError, TypeError, ValueError):
        return None
    return fang_pb2.ConfusionMatrix(source=source, **counts)


def _read_dataset(run: RunInfo) -> tuple[dict[str, Any], dict[str, fang_pb2.ConfusionMatrix]]:
    """The training dataset summary and per-model confusion counts of a run.

    Read from the run's JSON sidecars so the pickled bundle is never loaded on a request path.
    ``training_report.json`` carries the dataset and cross-fit counts; the nested evaluation's
    pooled held-out counts, when the run has them, replace the cross-fit ones per model.
    Raises ValueError when the report is missing or carries no dataset summary.
    """
    report = _read_json_mapping(run.path / "training_report.json")
    dataset = report.get("dataset") if report else None
    if report is None or not isinstance(dataset, dict):
        message = f"Run {run.run_id!r} has no readable training_report.json dataset."
        raise ValueError(message)

    matrices: dict[str, fang_pb2.ConfusionMatrix] = {}
    models = report.get("models")
    for name, entry in models.items() if isinstance(models, dict) else ():
        crossfit = entry.get("crossfit_metrics") if isinstance(entry, dict) else None
        raw = crossfit.get("confusion_matrix_counts") if isinstance(crossfit, dict) else None
        if (matrix := _count_matrix(raw, "cross_fit")) is not None:
            matrices[str(name)] = matrix

    evaluation = _read_json_mapping(run.path / "evaluation" / "metrics.json")
    pooled = evaluation.get("pooled_metrics") if evaluation else None
    for name, entry in pooled.items() if isinstance(pooled, dict) else ():
        raw = entry.get("confusion_matrix_counts") if isinstance(entry, dict) else None
        if (matrix := _count_matrix(raw, "held_out")) is not None:
            matrices[str(name)] = matrix
    return dataset, matrices


class FangServicer(fang_pb2_grpc.FangServicer):
    def __init__(self, artifact_dir: Path | str) -> None:
        self._artifact_dir = Path(artifact_dir)

    def GetVersion(self, request, context):
        return fang_pb2.GetVersionResponse(
            library_version=library_version,
            service_version=SERVICE_VERSION,
        )

    def ListRuns(self, request, context):
        runs = [
            fang_pb2.RunInfo(
                run_id=run.run_id,
                created_at=run.created_at,
                schema_version=run.schema_version,
                selected_model=run.selected_model,
                threshold=run.threshold,
            )
            for run in list_runs(self._artifact_dir)
        ]
        return fang_pb2.ListRunsResponse(runs=runs)

    def GetRunSettings(self, request, context):
        try:
            run = _resolve_run(self._artifact_dir, request.run_id)
        except FileNotFoundError as error:
            context.abort(grpc.StatusCode.NOT_FOUND, str(error))

        try:
            best_params = _read_best_params(run)
        except ValueError as error:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(error))

        return fang_pb2.GetRunSettingsResponse(
            run_id=run.run_id,
            selected_model=run.selected_model,
            threshold=run.threshold,
            best_params=to_struct(best_params),
        )

    def GetRunDataset(self, request, context):
        try:
            run = _resolve_run(self._artifact_dir, request.run_id)
        except FileNotFoundError as error:
            context.abort(grpc.StatusCode.NOT_FOUND, str(error))

        try:
            dataset, matrices = _read_dataset(run)
        except ValueError as error:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(error))

        def count(key: str) -> int:
            value = dataset.get(key)
            return int(value) if isinstance(value, (int, float)) else 0

        status_counts = dataset.get("status_counts")
        return fang_pb2.GetRunDatasetResponse(
            run_id=run.run_id,
            selected_model=run.selected_model,
            n_rows=count("n_rows"),
            n_stars=count("n_stars"),
            n_files=count("n_files"),
            n_features=count("n_features"),
            n_positive=count("n_positive"),
            n_negative=count("n_negative"),
            n_unlabelled=count("n_unlabelled"),
            status_counts={
                str(k): int(v)
                for k, v in (status_counts.items() if isinstance(status_counts, dict) else ())
            },
            confusion_matrices=matrices,
        )

    def Predict(self, request, context):
        rows = [from_struct(item) for item in request.features]
        if not rows:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "Predict requires at least one feature row.",
            )

        try:
            run = _resolve_run(self._artifact_dir, request.run_id)
        except FileNotFoundError as error:
            context.abort(grpc.StatusCode.NOT_FOUND, str(error))

        try:
            predictions = predict_features(
                rows,
                run.path,
                star_id=request.star_id or "unknown",
            )
        except DataValidationError as error:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))
        except SchemaVersionError as error:
            # Only loading the model raises this: the deployed model does not match
            # the installed schema. That is the server's fault, not the caller's.
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, str(error))

        return fang_pb2.PredictResponse(
            predictions=[to_struct(row) for row in predictions],
            model_run_id=run.run_id,
            schema_version=run.schema_version,
        )
