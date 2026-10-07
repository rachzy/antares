"""The server wiring: long extractions must not starve the short RPCs."""

from __future__ import annotations

import threading

import grpc
from antares.shaula.v1 import shaula_pb2, shaula_pb2_grpc
from shaula.api import ProgressEvent, ResolvedTarget

from shaula_service import servicer as servicer_module
from shaula_service.server import build_server


def test_resolve_target_is_answered_while_every_extraction_slot_is_busy(monkeypatch):
    release = threading.Event()

    def blocking_extract(target, mission, *, progress=None, **kwargs):
        progress(ProgressEvent(stage="downloading", message="started"))
        release.wait(10)
        raise RuntimeError("released")

    monkeypatch.setattr(servicer_module, "extract", blocking_extract)
    monkeypatch.setattr(
        servicer_module,
        "resolve",
        lambda query, mission: ResolvedTarget(
            catalog="KIC", catalog_id="1", ra_deg=1.0, dec_deg=2.0, display_name="x"
        ),
    )

    server, port = build_server("127.0.0.1:0", extraction_slots=1)
    server.start()
    try:
        with grpc.insecure_channel(f"127.0.0.1:{port}") as channel:
            stub = shaula_pb2_grpc.ShaulaStub(channel)
            stream = stub.ExtractFeatures(
                shaula_pb2.ExtractFeaturesRequest(target="KIC 1", mission="Kepler")
            )
            next(stream)

            response = stub.ResolveTarget(
                shaula_pb2.ResolveTargetRequest(query="x", mission="Kepler"), timeout=3
            )

            assert response.designation == "KIC 1"
            stream.cancel()
    finally:
        release.set()
        server.stop(0)
