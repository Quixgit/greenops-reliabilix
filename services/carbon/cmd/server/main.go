package main

import (
	"net/http"

	"github.com/quixgit/greenops-reliabilix/pkg/service"
	"github.com/quixgit/greenops-reliabilix/services/carbon/internal/transport"
)

func main() {
	service.Run("carbon", "8085", service.Options{}, func(mux *http.ServeMux, _ service.Deps) {
		transport.Register(mux)
	})
}
