SERVICES := gateway tenant project cloudintegration ingestion carbon

.PHONY: build test lint vuln up down proto
build: ; $(foreach s,$(SERVICES),go build -o bin/$(s) ./services/$(s)/cmd/server;)
test:  ; go test -race ./...
lint:  ; golangci-lint run ./...
vuln:  ; govulncheck ./...
proto: ; cd proto && buf lint && buf generate
up:    ; docker compose -f deploy/docker/docker-compose.yml up -d
down:  ; docker compose -f deploy/docker/docker-compose.yml down

# make run-carbon  -> run a single service locally with the dev verifier
run-%: ; ENV=dev go run ./services/$*/cmd/server
