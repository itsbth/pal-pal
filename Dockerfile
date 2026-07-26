# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26.5-bookworm AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/pal-pal ./cmd/pal-pal && \
    install -d -m 0750 /out/data

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build --chown=65532:65532 /out/pal-pal /pal-pal
COPY --from=build --chown=65532:65532 /out/data /data

ENV DATA_PATH=/data \
    LISTEN_ADDRESS=:8080

EXPOSE 8080
STOPSIGNAL SIGTERM

USER nonroot:nonroot
ENTRYPOINT ["/pal-pal"]
CMD ["serve"]
