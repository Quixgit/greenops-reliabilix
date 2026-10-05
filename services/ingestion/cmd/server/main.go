package main

import (
	"net/http"

	"github.com/quixgit/greenops-reliabilix/pkg/service"
)

func main() {
	// TODO: subscribe to greenops.cloud.sync_completed.v1, normalize, persist, publish usage.normalized.
	service.Run("ingestion", "8084", service.Options{}, func(mux *http.ServeMux, _ service.Deps) {})
}
