# syntax=docker/dockerfile:1
# One image per process:  docker build --build-arg CMD=api -f deploy/docker/Dockerfile .   (api | worker | scheduler)
FROM golang:1.27 AS build
ARG CMD
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${CMD}

# Migrations runner:  docker build --target migrate -f deploy/docker/Dockerfile .
FROM golang:1.27 AS goose
RUN CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@latest

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
COPY --from=goose /go/bin/goose /goose
COPY backend/migrations /migrations
ENV GOOSE_DRIVER=postgres GOOSE_MIGRATION_DIR=/migrations
ENTRYPOINT ["/goose"]
CMD ["up"]

# Default (last) stage: the service binary.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
