FROM golang:1.27-bookworm AS build

RUN apt-get update && apt-get install -y --no-install-recommends protobuf-compiler libprotobuf-dev \
    && rm -rf /var/lib/apt/lists/*
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

WORKDIR /src
COPY libs/proto ./libs/proto
RUN cd libs/proto && mkdir -p gen/go && protoc -I . \
      --go_out=gen/go --go_opt=module=github.com/rachzy/antares/libs/proto/gen/go \
      --go-grpc_out=gen/go --go-grpc_opt=module=github.com/rachzy/antares/libs/proto/gen/go \
      antares/shaula/v1/shaula.proto antares/fang/v1/fang.proto

COPY apps/api ./apps/api
RUN cd apps/api && CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /api
COPY --from=build --chown=65532:65532 /out/data /var/lib/antares

ENV ANTARES_ADDR=":8080" \
    ANTARES_DB_PATH="/var/lib/antares/antares.db"

VOLUME ["/var/lib/antares"]
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/api"]
