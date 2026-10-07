"""gRPC server entrypoint for the Fang service."""

from __future__ import annotations

import logging
import os
from concurrent import futures
from pathlib import Path

import grpc
from antares.fang.v1 import fang_pb2_grpc

from .servicer import FangServicer

logger = logging.getLogger(__name__)

DEFAULT_ADDRESS = "127.0.0.1:50052"
DEFAULT_ARTIFACT_DIR = "/var/lib/fang/models"


def serve(address: str, artifact_dir: Path | str) -> None:
    """Run until terminated. Prediction is sub-second, so the pool stays small."""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    fang_pb2_grpc.add_FangServicer_to_server(FangServicer(artifact_dir), server)
    server.add_insecure_port(address)
    server.start()
    logger.info("fang service listening on %s", address)
    server.wait_for_termination()


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    serve(
        os.environ.get("FANG_SERVICE_ADDRESS", DEFAULT_ADDRESS),
        os.environ.get("FANG_ARTIFACT_DIR", DEFAULT_ARTIFACT_DIR),
    )


if __name__ == "__main__":
    main()
