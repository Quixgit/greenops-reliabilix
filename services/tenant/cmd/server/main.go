package main

import (
	"net/http"

	"github.com/quixgit/greenops-reliabilix/pkg/service"
	"github.com/quixgit/greenops-reliabilix/services/tenant/internal/transport"
)

func main() {
	service.Run("tenant", "8081", service.Options{}, func(mux *http.ServeMux, _ service.Deps) {
		transport.Register(mux)
	})
}
