# syntax=docker/dockerfile:1
# One Dockerfile for every Go service:  docker build --build-arg SERVICE=carbon -f deploy/docker/Dockerfile .
FROM golang:1.27 AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./services/${SERVICE}/cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
