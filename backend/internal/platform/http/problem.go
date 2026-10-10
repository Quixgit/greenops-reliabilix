package httpx

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/i18n"
)

// Problem is an RFC 9457 problem details body.
type Problem struct {
	// Code is the stable, language-independent identifier of the problem (the English title as a slug). Clients translate
	// by Code; Title is already in the language of the request when the server has a translation.
	Code      string `json:"code,omitempty"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	// Errors maps a field (for example a setting key) to what is wrong with it: an RFC 9457 extension member.
	Errors map[string]string `json:"errors,omitempty"`
}

// WriteValidation answers 422 with one message per rejected field.
func WriteValidation(w http.ResponseWriter, r *http.Request, title string, errs map[string]string) {
	code, title := localize(r, title)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(Problem{Code: code, Title: title, Status: http.StatusUnprocessableEntity, RequestID: RequestID(r.Context()), Errors: errs})
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// localize returns the stable code of a title and the title in the request's language when a translation exists.
func localize(r *http.Request, title string) (code, out string) {
	code = strings.Trim(nonWord.ReplaceAllString(strings.ToLower(title), "_"), "_")
	out = title
	if lang := i18n.Negotiate(r.Header.Get("Accept-Language")); lang != i18n.Default {
		if t, ok := i18n.Lookup(lang, "problem."+code); ok {
			out = t
		}
	}
	return code, out
}

// WriteProblem writes an application/problem+json response.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	code, title := localize(r, title)
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Add("Vary", "Accept-Language")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{Code: code, Title: title, Status: status, Detail: detail, RequestID: RequestID(r.Context())})
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
