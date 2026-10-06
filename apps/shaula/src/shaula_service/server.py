"""gRPC server entrypoint for the Shaula service."""

from __future__ import annotations

import logging
import os
from concurrent import futures

import grpc
from antares.shaula.v1 import shaula_pb2_grpc

from .servicer import ShaulaServicer

logger = logging.getLogger(__name__)

DEFAULT_ADDRESS = "127.0.0.1:50051"
EXTRACTION_SLOTS = 4
CONTROL_WORKERS = 4


def build_server(
    address: str, extraction_slots: int = EXTRACTION_SLOTS
) -> tuple[grpc.Server, int]:
    """Build the server and return it with its bound port.

    One extraction occupies a worker for minutes, so the caller's queue should
    admit no more than ``extraction_slots`` jobs. The extra workers keep the
    short RPCs answerable while every extraction slot is busy.
    """
    pool = futures.ThreadPoolExecutor(max_workers=extraction_slots + CONTROL_WORKERS)
    server = grpc.server(pool)
    shaula_pb2_grpc.add_ShaulaServicer_to_server(ShaulaServicer(), server)
    return server, server.add_insecure_port(address)


def serve(address: str) -> None:
    """Run until terminated."""
    server, _ = build_server(address)
    server.start()
    logger.info("shaula service listening on %s", address)
    server.wait_for_termination()


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    serve(os.environ.get("SHAULA_SERVICE_ADDRESS", DEFAULT_ADDRESS))


if __name__ == "__main__":
    main()
