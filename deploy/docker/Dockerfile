# syntax=docker/dockerfile:1
# One image per process:  docker build --build-arg CMD=api -f deploy/docker/Dockerfile .   (api | worker | scheduler)
FROM golang:1.27.2 AS build
ARG CMD
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# The binary learns its identity at build time (platform/version): printed at start-up, served at /version, exported as build_info.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w \
    -X github.com/quixgit/greenops-reliabilix/backend/internal/platform/version.Platform=${VERSION} \
    -X github.com/quixgit/greenops-reliabilix/backend/internal/platform/version.Service=${CMD} \
    -X github.com/quixgit/greenops-reliabilix/backend/internal/platform/version.Commit=${COMMIT}" -o /out/app ./cmd/${CMD}

# Migrations runner:  docker build --target migrate -f deploy/docker/Dockerfile .
FROM golang:1.27.2 AS goose
RUN CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@latest

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
ARG VERSION=dev
ARG COMMIT=unknown
LABEL org.opencontainers.image.title="reliabilix-migrate" org.opencontainers.image.version="${VERSION}" org.opencontainers.image.revision="${COMMIT}" org.opencontainers.image.source="https://github.com/Quixgit/greenops-reliabilix"
COPY --from=goose /go/bin/goose /goose
COPY backend/migrations /migrations
ENV GOOSE_DRIVER=postgres GOOSE_MIGRATION_DIR=/migrations
ENTRYPOINT ["/goose"]
CMD ["up"]

# Default (last) stage: the service binary.
FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=dev
ARG COMMIT=unknown
ARG CMD
LABEL org.opencontainers.image.title="reliabilix-${CMD}" org.opencontainers.image.version="${VERSION}" org.opencontainers.image.revision="${COMMIT}" org.opencontainers.image.source="https://github.com/Quixgit/greenops-reliabilix"
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
