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


def serve(address: str) -> None:
    """Run until terminated.

    One extraction occupies a worker for minutes, so the pool size is the
    real concurrency limit of this machine. The caller's queue should admit
    no more than this many jobs at once.
    """
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    shaula_pb2_grpc.add_ShaulaServicer_to_server(ShaulaServicer(), server)
    server.add_insecure_port(address)
    server.start()
    logger.info("shaula service listening on %s", address)
    server.wait_for_termination()


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    serve(os.environ.get("SHAULA_SERVICE_ADDRESS", DEFAULT_ADDRESS))


if __name__ == "__main__":
    main()
