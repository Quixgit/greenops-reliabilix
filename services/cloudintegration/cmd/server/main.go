package main

import (
	"net/http"

	"github.com/quixgit/greenops-reliabilix/pkg/service"
	"github.com/quixgit/greenops-reliabilix/services/cloudintegration/internal/domain"
	"github.com/quixgit/greenops-reliabilix/services/cloudintegration/internal/infrastructure/aws"
)

func main() {
	registry := domain.NewRegistry(aws.Provider{})
	_ = registry // wired into application layer once sync use-cases exist
	service.Run("cloud-integration", "8083", service.Options{}, func(mux *http.ServeMux, _ service.Deps) {})
}
