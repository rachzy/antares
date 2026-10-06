"""Translation between the Fang proto contract and the Fang library."""

from __future__ import annotations

from pathlib import Path

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
