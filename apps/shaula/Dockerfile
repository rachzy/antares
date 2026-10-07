FROM python:3.14-slim AS build

# git is required: shaula is installed from a git tag.
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /build
COPY apps/shaula/pyproject.toml apps/shaula/uv.lock ./
RUN uv sync --frozen --no-install-project

COPY libs/proto ../../libs/proto
COPY apps/shaula/src ./src
RUN mkdir -p src/gen \
    && .venv/bin/python -m grpc_tools.protoc -I ../../libs/proto \
      --python_out=src/gen --grpc_python_out=src/gen \
      antares/shaula/v1/shaula.proto \
    && find src/gen -type d -exec touch {}/__init__.py \;

FROM python:3.14-slim AS runtime

COPY --from=build /build/.venv /app/.venv
COPY --from=build /build/src /app/src

ENV PATH="/app/.venv/bin:$PATH" \
    PYTHONPATH="/app/src:/app/src/gen" \
    SHAULA_LIGHTCURVE_CACHE="/var/cache/shaula/lightkurve" \
    SHAULA_SERVICE_ADDRESS="0.0.0.0:50051"

VOLUME ["/var/cache/shaula/lightkurve"]
EXPOSE 50051
CMD ["python", "-m", "shaula_service.server"]
