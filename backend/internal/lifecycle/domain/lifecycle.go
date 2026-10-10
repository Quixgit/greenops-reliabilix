// Package domain holds the data-lifecycle rules: what an export contains, when a retention window ends and who may
// confirm an erasure (ADR-0027). It has no database, HTTP or queue code.
package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNameMismatch     = errors.New("the typed name does not match the organization")
	ErrAlreadyScheduled = errors.New("an erasure is already scheduled")
	ErrNothingScheduled = errors.New("no erasure is scheduled")
)

// Deletion is the state of an organization's erasure request.
type Deletion struct {
	Scheduled    bool      `json:"scheduled"`
	RequestedBy  string    `json:"requested_by,omitempty"`
	RequestedAt  time.Time `json:"requested_at,omitempty"`
	ExecuteAfter time.Time `json:"execute_after,omitempty"`
}

// ConfirmName reports whether typed is the organization's exact name (surrounding space ignored, case kept): a
// deliberate act, not a click.
func ConfirmName(org, typed string) bool {
	return strings.TrimSpace(typed) != "" && strings.TrimSpace(typed) == strings.TrimSpace(org)
}

// MinRetentionMonths is the shortest retention that is ever applied: period comparisons and the year-over-year view
// need a full year, so a smaller configured value is treated as this one.
const MinRetentionMonths = 13

// RetentionCutoff returns the date before which whole monthly partitions may be dropped. ok is false when retention
// is off (months <= 0). The current month is never dropped: the cutoff is the first day of the month `months` back.
func RetentionCutoff(now time.Time, months int) (cutoff time.Time, ok bool) {
	if months <= 0 {
		return time.Time{}, false
	}
	months = max(months, MinRetentionMonths)
	first := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, -months, 0), true
}

// ExportTable is one table in the data export. Columns are listed explicitly (never "everything") so a secret column
// added later is not exported by accident; SQL is a plain SELECT run inside the organization's own transaction.
type ExportTable struct {
	Name string
	SQL  string
}

func tbl(name, cols, from string) ExportTable {
	return ExportTable{Name: name, SQL: "SELECT to_jsonb(t) FROM (SELECT " + cols + " FROM " + from + ") t"}
}

// ExportTables is the allow-list. Not exported on purpose: password-like material (API key hashes, invitation token
// hashes, the sealed notification target, the AWS ExternalId), other organizations' data and platform internals.
var ExportTables = []ExportTable{
	tbl("organization", "id, name, created_at", "tenants.organizations"),
	tbl("members", "m.id, u.email, m.role, m.created_at", "tenants.memberships m JOIN tenants.users u ON u.id = m.user_id"),
	tbl("invitations", "id, email, role, created_at, expires_at, accepted_at", "tenants.invitations"),
	tbl("api_keys", "id, name, prefix, role, created_at, last_used_at, revoked_at", "tenants.api_keys"),
	tbl("projects", "id, name, functional_unit, created_at", "projects.projects"),
	tbl("project_policies", "project_id, allowed_regions, ci_carbon_warn_kg, ci_carbon_block_kg, ci_cost_warn, ci_cost_block, updated_at", "projects.policies"),
	tbl("functional_units", "id, project_id, period_start, period_end, units", "projects.functional_unit_counts"),
	tbl("cloud_connections", "id, project_id, provider, account_ref, credential_ref, sync_status, last_error, last_sync_at, synced_through, created_at, export_bucket, export_prefix, export_name, export_region, last_quality", "cloudaccounts.connections"),
	tbl("sync_runs", "id, connection_id, started_at, finished_at, status, records, error, quality", "cloudaccounts.sync_runs"),
	tbl("usage_records", "id, project_id, connection_id, source, provider, resource_id, resource_type, service_name, service_category, region_id, consumed_quantity, consumed_unit, billed_cost, effective_cost, currency, recorded_at, charge_period_end", "usage.usage_records"),
	tbl("carbon_calculations", "id, project_id, provider, region_id, service_name, service_category, method, period_start, period_end, energy_kwh, intensity_g_per_kwh, embodied_g, carbon_kg_co2e, functional_units, sci_score, methodology_version, calculated_at", "carbon.calculations"),
	tbl("budgets", "id, project_id, amount, currency, period, created_at", "finops.budgets"),
	tbl("recommendations", "id, project_id, type, title, provider, service_name, current_region, recommended_region, estimated_carbon_reduction_pct, carbon_reduction_kg_month, estimated_cost_impact, cost_basis, confidence, compliance_check, status, decided_by, decided_at, applied_at, created_at, details", "recommendations.recommendations"),
	tbl("impact_verifications", "id, recommendation_id, project_id, outcome, method, baseline_from, baseline_to, after_from, after_to, baseline_days, after_days, measured_carbon_kg_month, measured_cost_month, expected_carbon_kg_month, expected_cost_month, realization_pct, currency, note, verified_at", "recommendations.impact_verifications"),
	tbl("automation_jobs", "id, project_id, recommendation_id, kind, status, plan, risk_level, risk_factors, rollback_plan, created_by, created_at, approved_by, approved_at, finished_by, finished_at, result_note", "automation.jobs"),
	tbl("reports", "id, project_id, kind, format, period_start, period_end, status, requested_by, created_at, completed_at", "reports.reports"),
	tbl("notification_destinations", "id, kind, name, target_hint, event_types, min_severity, enabled, created_by, created_at", "notifications.destinations"),
	tbl("settings", "key, value, source, updated_by, updated_at", "settings.tenant_values"),
	tbl("audit_log", "id, actor_user_id, action, target, request_id, metadata, created_at", "audit.audit_logs"),
}
