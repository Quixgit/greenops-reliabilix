// Package transport exposes the carbon service over HTTP.
package transport

import (
	"errors"
	"net/http"

	"github.com/quixgit/greenops-reliabilix/pkg/auth"
	"github.com/quixgit/greenops-reliabilix/pkg/httpx"
	"github.com/quixgit/greenops-reliabilix/services/carbon/internal/domain"
)

type calculateRequest struct {
	Kind                 domain.UsageKind `json:"kind"`
	Amount               float64          `json:"amount"`
	GridIntensityGPerKWh float64          `json:"grid_intensity_g_per_kwh"`
}

// Register mounts carbon routes.
func Register(mux *http.ServeMux) {
	mux.Handle("POST /v1/carbon/calculate", auth.Require(auth.PermCarbonCompute)(http.HandlerFunc(calculate)))
}

func calculate(w http.ResponseWriter, r *http.Request) {
	var req calculateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid request body", "")
		return
	}
	res, err := domain.Calculate(domain.Input(req))
	if errors.Is(err, domain.ErrInvalidInput) {
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "invalid input", err.Error())
		return
	}
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
