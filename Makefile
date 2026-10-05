DB_OWNER_DSN ?= postgres://greenops_owner:dev-only@localhost:5432/greenops?sslmode=disable
DB_API_DSN   ?= postgres://greenops_api:dev-only@localhost:5432/greenops?sslmode=disable
DEV_TENANT   ?= 00000000-0000-4000-8000-000000000001

.PHONY: help up down build test test-integration lint vuln migrate seed seed-demo run-api run-worker run-scheduler web web-test
help: ; @grep -E '^[a-z-]+:' Makefile | cut -d: -f1 | tr '\n' ' '; echo

up:               ; docker compose up --build -d          # whole stack -> http://localhost:3000
down:             ; docker compose down

build:            ; cd backend && go build ./...
test:             ; cd backend && go test -race ./...
# needs migrated PostgreSQL: docker compose up -d postgres && make migrate
test-integration: ; cd backend && TEST_OWNER_DSN='$(DB_OWNER_DSN)' TEST_API_DSN='$(DB_API_DSN)' \
                      TEST_WORKER_DSN='postgres://greenops_worker:dev-only@localhost:5432/greenops?sslmode=disable' \
                      go test -tags integration -count=1 ./tests/...
lint:             ; cd backend && golangci-lint run ./...
vuln:             ; cd backend && govulncheck ./...

migrate:          ; cd backend && go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres '$(DB_OWNER_DSN)' up
seed:             ; psql '$(DB_OWNER_DSN)' -v ON_ERROR_STOP=1 -f deploy/dev/seed.sql
# DEV ONLY synthetic data (one AWS account, 60 days) so every Overview widget can be seen populated
seed-demo:        ; psql '$(DB_OWNER_DSN)' -v ON_ERROR_STOP=1 -f deploy/dev/seed-demo.sql

run-api:          ; cd backend && ENV=dev go run ./cmd/api
run-worker:       ; cd backend && ENV=dev go run ./cmd/worker
run-scheduler:    ; cd backend && ENV=dev go run ./cmd/scheduler
web:              ; cd frontend && npm run dev
web-test:         ; cd frontend && npm run typecheck && npm test
