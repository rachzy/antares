FROM python:3.14-slim AS build

# git is required: fang is installed from a git tag.
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /build
COPY apps/fang/pyproject.toml apps/fang/uv.lock ./
RUN uv sync --frozen --no-install-project

COPY libs/proto ../../libs/proto
COPY apps/fang/src ./src
RUN mkdir -p src/gen \
    && .venv/bin/python -m grpc_tools.protoc -I ../../libs/proto \
      --python_out=src/gen --grpc_python_out=src/gen \
      antares/fang/v1/fang.proto \
    && find src/gen -type d -exec touch {}/__init__.py \;

FROM python:3.14-slim AS runtime

# lightgbm links against OpenMP, which the slim base image lacks.
RUN apt-get update && apt-get install -y --no-install-recommends libgomp1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /build/.venv /app/.venv
COPY --from=build /build/src /app/src

ENV PATH="/app/.venv/bin:$PATH" \
    PYTHONPATH="/app/src:/app/src/gen" \
    FANG_ARTIFACT_DIR="/var/lib/fang/models" \
    FANG_SERVICE_ADDRESS="0.0.0.0:50052"

VOLUME ["/var/lib/fang/models"]
EXPOSE 50052
CMD ["python", "-m", "fang_service.server"]
