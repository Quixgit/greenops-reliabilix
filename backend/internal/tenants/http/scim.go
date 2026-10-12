package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

// SCIM 2.0 (RFC 7643/7644) provisioning of people for an identity provider (ADR-0056). Authenticated by an API key of role `scim`;
// the tenant is the key's own organization, so one key can only ever touch its own people.
const (
	scimUserSchema  = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimListSchema  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimErrorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"
	scimMaxCount    = 200
)

func (h Handlers) scimRoutes(r chi.Router) {
	r.Route("/scim/v2", func(r chi.Router) {
		r.Use(auth.Require(auth.PermSCIM))
		r.Get("/ServiceProviderConfig", h.scimConfig)
		r.Get("/ResourceTypes", h.scimResourceTypes)
		r.Get("/Users", h.scimList)
		r.Post("/Users", h.scimCreate)
		r.Get("/Users/{id}", h.scimGet)
		r.Put("/Users/{id}", h.scimReplace)
		r.Patch("/Users/{id}", h.scimPatch)
		r.Delete("/Users/{id}", h.scimDelete)
	})
}

func scimWrite(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func scimFail(w http.ResponseWriter, status int, scimType, detail string) {
	e := map[string]any{"schemas": []string{scimErrorSchema}, "status": strconv.Itoa(status), "detail": detail}
	if scimType != "" {
		e["scimType"] = scimType
	}
	scimWrite(w, status, e)
}

// scimError maps a failure to a SCIM error; details never include internals.
func scimError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		scimFail(w, 404, "", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		scimFail(w, 409, "uniqueness", "a user with this e-mail already exists")
	case errors.Is(err, domain.ErrInvalidInput):
		scimFail(w, 400, "invalidValue", "a valid e-mail and a role of viewer, engineer, admin or billing are required")
	case errors.Is(err, domain.ErrForbiddenChange), errors.Is(err, domain.ErrLastOwner):
		scimFail(w, 409, "mutability", "an owner cannot be removed by provisioning")
	case errors.Is(err, entitlements.ErrLimitReached):
		scimFail(w, 403, "", "the organization has reached its member limit")
	case errors.As(err, new(domain.PolicyError)):
		scimFail(w, 400, "invalidValue", err.Error())
	case errors.Is(err, application.ErrSCIMOff):
		scimFail(w, 501, "", err.Error())
	default:
		scimFail(w, 500, "", "internal error")
	}
}

func scimLocation(r *http.Request, id string) string {
	return strings.TrimSuffix(r.URL.Path, "/") + "/" + id
}

func scimOut(r *http.Request, u domain.SCIMUser, withLocation bool) map[string]any {
	m := map[string]any{
		"schemas": []string{scimUserSchema}, "id": u.ID, "userName": u.Email, "displayName": u.DisplayName, "active": u.Active,
		"name":   map[string]any{"formatted": u.DisplayName},
		"emails": []map[string]any{{"value": u.Email, "primary": true, "type": "work"}},
		"roles":  []map[string]any{{"value": string(u.Role), "primary": true}},
		"meta":   map[string]any{"resourceType": "User", "created": u.CreatedAt, "lastModified": u.UpdatedAt},
	}
	if u.ExternalID != "" {
		m["externalId"] = u.ExternalID
	}
	if withLocation {
		m["meta"].(map[string]any)["location"] = scimLocation(r, u.ID)
	}
	return m
}

type scimIn struct {
	UserName    string `json:"userName"`
	ExternalID  string `json:"externalId"`
	DisplayName string `json:"displayName"`
	Name        struct {
		Formatted  string `json:"formatted"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
	Emails []struct {
		Value   string `json:"value"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
	Active *scimBool `json:"active"`
	Roles  []struct {
		Value string `json:"value"`
	} `json:"roles"`
}

// scimBool reads a JSON boolean or the strings "True"/"False" that Azure AD sends.
type scimBool bool

func (b *scimBool) UnmarshalJSON(d []byte) error {
	switch strings.ToLower(strings.Trim(string(d), `"`)) {
	case "null":
		return nil
	case "true":
		*b = true
	case "false":
		*b = false
	default:
		return errors.New("not a boolean")
	}
	return nil
}

func (in scimIn) apply(u domain.SCIMUser) domain.SCIMUser {
	email := strings.TrimSpace(in.UserName)
	if !domain.ValidEmail(email) {
		email = ""
		for _, e := range in.Emails {
			if e.Primary && domain.ValidEmail(strings.TrimSpace(e.Value)) {
				email = strings.TrimSpace(e.Value)
			}
		}
		if email == "" && len(in.Emails) > 0 {
			email = strings.TrimSpace(in.Emails[0].Value)
		}
	}
	if email != "" {
		u.Email = email
	}
	name := strings.TrimSpace(in.DisplayName)
	if name == "" {
		name = strings.TrimSpace(in.Name.Formatted)
	}
	if name == "" {
		name = strings.TrimSpace(in.Name.GivenName + " " + in.Name.FamilyName)
	}
	u.DisplayName = name
	u.ExternalID = in.ExternalID
	if in.Active != nil {
		u.Active = bool(*in.Active)
	}
	vals := make([]string, 0, len(in.Roles))
	for _, r := range in.Roles {
		vals = append(vals, r.Value)
	}
	u.Role = domain.SCIMRoleFrom(vals)
	return u
}

func decodeSCIM(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		scimFail(w, 400, "invalidSyntax", "the body is not valid SCIM JSON")
		return false
	}
	return true
}

func (h Handlers) scimTenant(r *http.Request) string {
	c, _ := auth.FromContext(r.Context())
	return c.TenantID
}

func (h Handlers) scimConfig(w http.ResponseWriter, _ *http.Request) {
	scimWrite(w, 200, map[string]any{
		"schemas":               []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch":                 map[string]any{"supported": true},
		"bulk":                  map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":                map[string]any{"supported": true, "maxResults": scimMaxCount},
		"changePassword":        map[string]any{"supported": false},
		"sort":                  map[string]any{"supported": false},
		"etag":                  map[string]any{"supported": false},
		"authenticationSchemes": []map[string]any{{"type": "oauthbearertoken", "name": "Bearer token", "description": "An API key with the scim role"}},
	})
}

func (h Handlers) scimResourceTypes(w http.ResponseWriter, _ *http.Request) {
	scimWrite(w, 200, map[string]any{"schemas": []string{scimListSchema}, "totalResults": 1, "startIndex": 1, "itemsPerPage": 1,
		"Resources": []map[string]any{{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"}, "id": "User", "name": "User", "endpoint": "/Users", "schema": scimUserSchema}}})
}

// scimFilterEmail understands the one filter identity providers use to look a person up: userName eq "x".
func scimFilterEmail(filter string) (string, bool) {
	f := strings.Fields(filter)
	if len(f) < 3 || !strings.EqualFold(f[0], "userName") || !strings.EqualFold(f[1], "eq") {
		return "", false
	}
	v := strings.Trim(strings.Join(f[2:], " "), `"`)
	return v, v != ""
}

func (h Handlers) scimList(w http.ResponseWriter, r *http.Request) {
	email := ""
	if f := r.URL.Query().Get("filter"); f != "" {
		v, ok := scimFilterEmail(f)
		if !ok {
			scimFail(w, 400, "invalidFilter", `only the filter userName eq "address" is supported`)
			return
		}
		email = v
	}
	start, _ := strconv.Atoi(r.URL.Query().Get("startIndex"))
	if start < 1 {
		start = 1
	}
	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || count < 0 || count > scimMaxCount {
		count = scimMaxCount
	}
	users, total, err := h.Svc.SCIMList(r.Context(), h.scimTenant(r), email, start, count)
	if err != nil {
		scimError(w, err)
		return
	}
	res := make([]map[string]any, 0, len(users))
	for _, u := range users {
		res = append(res, scimOut(r, u, false))
	}
	scimWrite(w, 200, map[string]any{"schemas": []string{scimListSchema}, "totalResults": total, "startIndex": start, "itemsPerPage": len(res), "Resources": res})
}

func (h Handlers) scimGet(w http.ResponseWriter, r *http.Request) {
	u, err := h.Svc.SCIMGet(r.Context(), h.scimTenant(r), chi.URLParam(r, "id"))
	if err != nil {
		scimError(w, err)
		return
	}
	scimWrite(w, 200, scimOut(r, u, false))
}

func (h Handlers) scimCreate(w http.ResponseWriter, r *http.Request) {
	var in scimIn
	if !decodeSCIM(w, r, &in) {
		return
	}
	u, err := h.Svc.SCIMCreate(r.Context(), h.scimTenant(r), in.apply(domain.SCIMUser{Active: true}))
	if err != nil {
		scimError(w, err)
		return
	}
	w.Header().Set("Location", scimLocation(r, u.ID))
	scimWrite(w, 201, scimOut(r, u, true))
}

func (h Handlers) scimReplace(w http.ResponseWriter, r *http.Request) {
	var in scimIn
	if !decodeSCIM(w, r, &in) {
		return
	}
	id := chi.URLParam(r, "id")
	cur, err := h.Svc.SCIMGet(r.Context(), h.scimTenant(r), id)
	if err != nil {
		scimError(w, err)
		return
	}
	cur.Active = true // a replace that does not say otherwise leaves the person active
	u, err := h.Svc.SCIMUpdate(r.Context(), h.scimTenant(r), in.apply(cur))
	if err != nil {
		scimError(w, err)
		return
	}
	scimWrite(w, 200, scimOut(r, u, false))
}

type scimPatch struct {
	Operations []struct {
		Op    string          `json:"op"`
		Path  string          `json:"path"`
		Value json.RawMessage `json:"value"`
	} `json:"Operations"`
}

// scimPatchKey turns one (path, value) into the attribute object scimIn understands; unknown paths are ignored, as providers
// send attributes this service does not keep.
func scimPatchKey(path string, value json.RawMessage) map[string]json.RawMessage {
	path = strings.TrimSpace(path)
	if path == "" {
		var m map[string]json.RawMessage
		if json.Unmarshal(value, &m) != nil {
			return nil
		}
		return m
	}
	switch strings.ToLower(path) {
	case "active", "username", "displayname", "externalid", "roles", "emails":
		return map[string]json.RawMessage{map[string]string{"active": "active", "username": "userName", "displayname": "displayName", "externalid": "externalId", "roles": "roles", "emails": "emails"}[strings.ToLower(path)]: value}
	case "name.formatted":
		return map[string]json.RawMessage{"displayName": value}
	}
	return nil
}

func (h Handlers) scimPatch(w http.ResponseWriter, r *http.Request) {
	var p scimPatch
	if !decodeSCIM(w, r, &p) {
		return
	}
	id := chi.URLParam(r, "id")
	cur, err := h.Svc.SCIMGet(r.Context(), h.scimTenant(r), id)
	if err != nil {
		scimError(w, err)
		return
	}
	next := cur
	for _, op := range p.Operations {
		if o := strings.ToLower(op.Op); o != "replace" && o != "add" {
			continue // "remove" of an attribute is not meaningful for what is kept here
		}
		keys := scimPatchKey(op.Path, op.Value)
		if keys == nil {
			continue
		}
		raw, _ := json.Marshal(keys)
		var in scimIn
		if err := json.Unmarshal(raw, &in); err != nil {
			scimFail(w, 400, "invalidValue", "an attribute has an unusable value")
			return
		}
		// only the attributes this operation names change
		if _, ok := keys["active"]; ok && in.Active != nil {
			next.Active = bool(*in.Active)
		}
		if _, ok := keys["roles"]; ok {
			next.Role = in.apply(domain.SCIMUser{}).Role
		}
		if _, ok := keys["displayName"]; ok {
			next.DisplayName = strings.TrimSpace(in.DisplayName)
		}
		if _, ok := keys["externalId"]; ok {
			next.ExternalID = in.ExternalID
		}
		if _, ok := keys["userName"]; ok && domain.ValidEmail(strings.TrimSpace(in.UserName)) {
			next.Email = strings.TrimSpace(in.UserName)
		}
	}
	u, err := h.Svc.SCIMUpdate(r.Context(), h.scimTenant(r), next)
	if err != nil {
		scimError(w, err)
		return
	}
	scimWrite(w, 200, scimOut(r, u, false))
}

func (h Handlers) scimDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.Svc.SCIMDelete(r.Context(), h.scimTenant(r), chi.URLParam(r, "id")); err != nil {
		scimError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
