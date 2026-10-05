package main

import (
	"net/http"

	"github.com/quixgit/greenops-reliabilix/pkg/service"
	"github.com/quixgit/greenops-reliabilix/services/project/internal/infrastructure"
	"github.com/quixgit/greenops-reliabilix/services/project/internal/transport"
)

func main() {
	repo := infrastructure.NewMemoryRepo() // TODO: PostgreSQL adapter
	service.Run("project", "8082", service.Options{}, func(mux *http.ServeMux, _ service.Deps) {
		transport.Handler{Repo: repo}.Register(mux)
	})
}
