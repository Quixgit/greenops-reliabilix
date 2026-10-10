package httpx

import (
	"encoding/json"
	"net/http"
)

// Problem is an RFC 9457 problem details body.
type Problem struct {
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	// Errors maps a field (for example a setting key) to what is wrong with it: an RFC 9457 extension member.
	Errors map[string]string `json:"errors,omitempty"`
}

// WriteValidation answers 422 with one message per rejected field.
func WriteValidation(w http.ResponseWriter, r *http.Request, title string, errs map[string]string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(Problem{Title: title, Status: http.StatusUnprocessableEntity, RequestID: RequestID(r.Context()), Errors: errs})
}

// WriteProblem writes an application/problem+json response.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{Title: title, Status: status, Detail: detail, RequestID: RequestID(r.Context())})
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON strictly decodes the request body (unknown fields rejected).
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
