"""The Shaula servicer's translation between proto and library calls."""

from __future__ import annotations

import grpc
import pytest
from antares.shaula.v1 import shaula_pb2
from shaula.api import ResolvedTarget, TargetNotFound

from shaula_service import servicer as servicer_module
from shaula_service.servicer import ShaulaServicer


class _Context:
    def __init__(self) -> None:
        self.code: grpc.StatusCode | None = None
        self.details: str = ""

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
