package settings

import "sync"

// Keys referenced by code. A setting that code reads has a constant here, so a typo is a compile error.
const (
	KeyMaintenanceEnabled      = "platform.maintenance_enabled"
	KeyMaintenanceMessage      = "platform.maintenance_message"
	KeySignupsEnabled          = "platform.signups_enabled"
	KeySignupsAllowedDomains   = "platform.signups_allowed_email_domains"
	KeyNotifRetentionDays      = "notifications.retention_days"
	KeyNotifMaxAttempts        = "notifications.max_attempts"
	KeyNotifRetryBaseSeconds   = "notifications.retry_base_seconds"
	KeyNotifSendTimeoutSeconds = "notifications.send_timeout_seconds"
	KeyErrorsRetentionDays     = "errors.retention_days"
	KeyOperatorLogRetention    = "operator.access_log_retention_days"

	KeyNotifEnabled        = "notifications.enabled"
	KeyInviteExpiryDays    = "security.invitation_expiry_days"
	KeyInviteAllowedDomain = "security.invitation_allowed_domains"
	KeyAPIKeysMaxActive    = "security.api_keys_max_active"
	KeyReportFormats       = "reports.allowed_formats"
	KeySyncMinIntervalH    = "sync.min_interval_hours"

	KeyLimitProjects     = "limits.projects"
	KeyLimitConnections  = "limits.cloud_connections"
	KeyLimitMembers      = "limits.members"
	KeyLimitDestinations = "limits.notification_destinations"
	KeyProvidersAllowed  = "connectors.allowed_providers"

	KeyModuleAutomation      = "modules.automation"
	KeyModuleKubernetes      = "modules.kubernetes"
	KeyModuleReports         = "modules.reports"
	KeyModuleRecommendations = "modules.recommendations"
	KeyModuleNotifications   = "modules.notifications"
	KeyModuleCIGate          = "modules.ci_gate"
)

var groups = []Group{
	{"platform", "Platform", "Availability and onboarding of the whole deployment."},
	{"modules", "Modules", "Which product areas an organization can use."},
	{"limits", "Limits", "Quotas per organization. 0 means unlimited."},
	{"security", "Security", "Invitations and machine access."},
	{"connectors", "Cloud connectors", "Which cloud providers can be connected and how often data is pulled."},
	{"reports", "Reports", "What can be generated."},
	{"notifications", "Notifications", "Delivery of events by e-mail, webhook and Slack."},
	{"retention", "Retention", "How long operational records are kept."},
}

func intDef(key, group, title, desc string, scope Scope, def int64, min, max float64, unit, note string) Def {
	return Def{Key: key, Group: group, Title: title, Description: desc, Scope: scope, Type: Int, Default: def, Min: f(min), Max: f(max), Unit: unit, Note: note}
}

func boolDef(key, group, title, desc string, scope Scope, def bool, note string) Def {
	return Def{Key: key, Group: group, Title: title, Description: desc, Scope: scope, Type: Bool, Default: def, Note: note}
}

func moduleDef(key, title, desc string) Def {
	return boolDef(key, "modules", title, desc, ScopeOperator, true,
		"When off, every endpoint of the module answers 403 for this organization. Existing data is kept.")
}

func definitions() []Def {
	const domainRe = `^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`
	return []Def{
		// ---- platform (operators) ----
		boolDef(KeyMaintenanceEnabled, "platform", "Maintenance mode", "Reject all changes for organizations while the platform is being worked on. Reading still works.", ScopePlatform, false,
			"Answers 503 with Retry-After to every POST, PATCH and DELETE of organization users. The operator console keeps working."),
		{Key: KeyMaintenanceMessage, Group: "platform", Title: "Maintenance message", Description: "Shown to people whose change was rejected during maintenance.", Scope: ScopePlatform, Type: String,
			Default: "The platform is in maintenance. Please try again shortly.", MaxLen: 300},
		boolDef(KeySignupsEnabled, "platform", "Self-service sign-up", "Allow people without an organization to create one.", ScopePlatform, true,
			"When off, only invitations can add people to the platform."),
		{Key: KeySignupsAllowedDomains, Group: "platform", Title: "Sign-up e-mail domains", Description: "Only people with an e-mail address in these domains may create an organization. Empty allows every domain.",
			Scope: ScopePlatform, Type: List, Default: []string{}, MaxItems: 100, MaxLen: 100, Pattern: domainRe},

		// ---- modules (operators, per organization) ----
		moduleDef(KeyModuleAutomation, "Automation", "Change plans for approved recommendations."),
		moduleDef(KeyModuleKubernetes, "Kubernetes", "Kubernetes energy data (reserved for a later release)."),
		moduleDef(KeyModuleReports, "Reports", "Carbon, SCI, cost and sustainability reports."),
		moduleDef(KeyModuleRecommendations, "Recommendations", "Region shift and rightsizing recommendations."),
		moduleDef(KeyModuleNotifications, "Notifications", "Destinations and delivery of events."),
		moduleDef(KeyModuleCIGate, "CI gate", "The /ci/evaluate endpoint for pipelines."),

		// ---- limits (operators, per organization) ----
		intDef(KeyLimitProjects, "limits", "Projects", "Maximum number of projects.", ScopeOperator, 0, 0, 10000, "", "Existing projects are never removed when a limit is lowered; only new ones are refused."),
		intDef(KeyLimitConnections, "limits", "Cloud connections", "Maximum number of cloud connections.", ScopeOperator, 0, 0, 10000, "", ""),
		intDef(KeyLimitMembers, "limits", "Members", "Maximum number of members, counting invitations that are still open.", ScopeOperator, 0, 0, 100000, "", ""),
		intDef(KeyLimitDestinations, "limits", "Notification destinations", "Maximum number of notification destinations.", ScopeOperator, 20, 0, 200, "", "0 means the platform maximum of 200."),

		// ---- security (organization) ----
		intDef(KeyInviteExpiryDays, "security", "Invitation lifetime", "How long an invitation can be accepted.", ScopeTenant, 7, 1, 30, "days", "Applies to invitations created after the change."),
		{Key: KeyInviteAllowedDomain, Group: "security", Title: "Invitation e-mail domains", Description: "Only addresses in these domains can be invited. Empty allows every domain.",
			Scope: ScopeTenant, Type: List, Default: []string{}, MaxItems: 50, MaxLen: 100, Pattern: domainRe},
		intDef(KeyAPIKeysMaxActive, "security", "Active API keys", "Maximum number of API keys that are not revoked.", ScopeTenant, 10, 1, 50, "", ""),

		// ---- cloud connectors ----
		{Key: KeyProvidersAllowed, Group: "connectors", Title: "Allowed providers", Description: "Cloud providers this organization may connect.", Scope: ScopeOperator, Type: List,
			Default: []string{"aws", "gcp"}, ListEnum: []string{"aws", "gcp", "azure"}, MinItems: 1, MaxItems: 3, Note: "Azure is not available yet; listing it has no effect until the connector exists."},
		intDef(KeySyncMinIntervalH, "connectors", "Minimum hours between data pulls", "Each connection is pulled at most this often.", ScopeTenant, 6, 6, 168, "hours",
			"The platform pulls every 6 hours; a larger value skips runs. Lower values cannot make it faster."),

		// ---- reports ----
		{Key: KeyReportFormats, Group: "reports", Title: "Allowed report formats", Description: "Formats people in this organization can request.", Scope: ScopeTenant, Type: List,
			Default: []string{"csv", "json", "pdf"}, ListEnum: []string{"csv", "json", "pdf"}, MinItems: 1, MaxItems: 3, Note: "At least one format must remain."},

		// ---- notifications ----
		boolDef(KeyNotifEnabled, "notifications", "Send notifications", "Master switch: when off, events are not queued for any destination.", ScopeTenant, true, "Events published while it is off are not delivered later."),
		intDef(KeyNotifMaxAttempts, "notifications", "Delivery attempts", "How many times a delivery is tried before it is marked failed.", ScopePlatform, 8, 1, 20, "attempts", ""),
		intDef(KeyNotifRetryBaseSeconds, "notifications", "First retry delay", "Delay before the second attempt; it doubles each time up to one hour.", ScopePlatform, 30, 5, 600, "seconds", ""),
		intDef(KeyNotifSendTimeoutSeconds, "notifications", "Send timeout", "How long one send may take.", ScopePlatform, 20, 5, 60, "seconds", ""),

		// ---- retention (platform) ----
		intDef(KeyNotifRetentionDays, "retention", "Delivery history", "Finished notification deliveries are deleted after this many days.", ScopePlatform, 90, 7, 730, "days", ""),
		intDef(KeyErrorsRetentionDays, "retention", "Handled errors", "Resolved and ignored error groups are deleted after this many days.", ScopePlatform, 90, 7, 730, "days", ""),
		intDef(KeyOperatorLogRetention, "retention", "Operator access log", "Operator access records are deleted after this many days.", ScopePlatform, 365, 90, 3650, "days", "Kept long on purpose: it is the record of who looked at what."),
	}
}

var (
	once    sync.Once
	catalog *Catalog
	catErr  error
)

// Default returns the platform catalog. A broken definition panics at first use, which tests and startup exercise.
func Default() *Catalog {
	once.Do(func() { catalog, catErr = NewCatalog(groups, definitions()) })
	if catErr != nil {
		panic(catErr)
	}
	return catalog
}
