"""Translation between the Shaula proto contract and the Shaula library."""

from __future__ import annotations

import grpc
from antares.shaula.v1 import shaula_pb2, shaula_pb2_grpc
from shaula import __version__ as library_version
from shaula.api import TargetNotFound, resolve

SERVICE_VERSION = "0.0.1a0"


class ShaulaServicer(shaula_pb2_grpc.ShaulaServicer):
    def GetVersion(self, request, context):
        return shaula_pb2.GetVersionResponse(
            library_version=library_version,
            service_version=SERVICE_VERSION,
        )

    def ResolveTarget(self, request, context):
        try:
            resolved = resolve(request.query, request.mission or "Kepler")
        except TargetNotFound as error:
            context.abort(grpc.StatusCode.NOT_FOUND, str(error))
        except ValueError as error:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(error))

        return shaula_pb2.ResolveTargetResponse(
            catalog=resolved.catalog,
            catalog_id=resolved.catalog_id,
            ra_deg=resolved.ra_deg,
            dec_deg=resolved.dec_deg,
            display_name=resolved.display_name,
            designation=resolved.designation,
        )
