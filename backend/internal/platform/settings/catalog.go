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
	KeyAlertsWarnPct           = "alerts.warn_pct"
	KeyAlertsCarbonMonthlyKg   = "alerts.carbon_monthly_kg"
	KeyDataRetentionMonths     = "retention.data_months"
	KeyDeletionGraceDays       = "lifecycle.deletion_grace_days"

	KeyNotifEnabled        = "notifications.enabled"
	KeyInviteExpiryDays    = "security.invitation_expiry_days"
	KeyInviteAllowedDomain = "security.invitation_allowed_domains"
	KeyAPIKeysMaxActive    = "security.api_keys_max_active"
	KeyReportFormats       = "reports.allowed_formats"
	KeySyncMinIntervalH    = "sync.min_interval_hours"

	KeyDefaultLocale  = "i18n.default_locale"
	KeyEnabledLocales = "i18n.enabled_locales"

	KeyCarbonMaxRequests = "carbon.max_requests_per_day"
	KeyCarbonAutoLoad    = "carbon.auto_load_history"
	KeyCarbonAutoDays    = "carbon.auto_load_days"

	KeyReportingCurrency = "finops.reporting_currency"

	// Unit costs of serving an organization (USD): inputs of the console's estimated cost per organization.
	KeyCostBase       = "costs.base_per_organization_month"
	KeyCostUsageRows  = "costs.per_million_usage_rows"
	KeyCostCalcRows   = "costs.per_thousand_calculation_rows"
	KeyCostSyncRun    = "costs.per_sync_run"
	KeyCostDeliveries = "costs.per_thousand_deliveries"
	KeyCostReport     = "costs.per_report"
	KeyCostK8sSamples = "costs.per_million_k8s_samples"

	KeyCarbonPriority    = "carbon.provider_priority"
	KeyCarbonOverrides   = "carbon.region_provider_overrides"
	KeyCarbonMinReadings = "carbon.min_day_readings"
	KeyCarbonMaxAgeHours = "carbon.max_reading_age_hours"

	KeyOrgSuspended        = "organization.suspended"
	KeyOrgSuspendedMessage = "organization.suspended_message"

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
	{"organization", "Organization status", "Suspend an organization or restore it. Every change is recorded in the change history."},
	{"costs", "Cost of serving an organization", "Unit costs (USD) that turn an organization's resource use into an estimated monthly cost of serving it."},
	{"i18n", "Languages", "Which languages the platform offers and which one is used when a person has not chosen."},
	{"carbon", "Grid data sources", "Which source supplies the grid carbon intensity, and how much data a source needs before it is used."},
	{"reporting", "Cost reporting", "In which currency cost is shown when a bill has several."},
	{"modules", "Modules", "Which product areas an organization can use."},
	{"limits", "Limits", "Quotas per organization. 0 means unlimited."},
	{"security", "Security", "Invitations and machine access."},
	{"connectors", "Cloud connectors", "Which cloud providers can be connected and how often data is pulled."},
	{"reports", "Reports", "What can be generated."},
	{"notifications", "Notifications", "Delivery of events by e-mail, webhook and Slack."},
	{"alerts", "Alerts", "When the platform raises an alert for the organization."},
	{"retention", "Retention", "How long operational records are kept."},
}

func intDef(key, group, title, desc string, scope Scope, def int64, min, max float64, unit, note string) Def {
	return Def{Key: key, Group: group, Title: title, Description: desc, Scope: scope, Type: Int, Default: def, Min: f(min), Max: f(max), Unit: unit, Note: note}
}

func floatDef(key, group, title, desc string, def, min, max float64, unit, note string) Def {
	return Def{Key: key, Group: group, Title: title, Description: desc, Scope: ScopePlatform, Type: Float, Default: def, Min: f(min), Max: f(max), Unit: unit, Note: note}
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

		// ---- organization status (operator) ----
		boolDef(KeyOrgSuspended, "organization", "Suspended", "While on, people in this organization can read and export their data but cannot change anything.", ScopeOperator, false,
			"Data is kept. Turn it off to restore access immediately. Set the message below first: it is the reason people see."),
		{Key: KeyOrgSuspendedMessage, Group: "organization", Title: "Message shown while suspended", Description: "Shown to people whose change was rejected because the organization is suspended.",
			Scope: ScopeOperator, Type: String, Default: "This organization is suspended. Contact support to restore access.", MaxLen: 300},

		// ---- unit costs (platform) ----
		floatDef(KeyCostBase, "costs", "Fixed cost per organization", "Monthly overhead of having an organization (monitoring, support, backups).", 0, 0, 100000, "USD", "Set from your own invoices. The defaults below are placeholders, not measured costs."),
		floatDef(KeyCostUsageRows, "costs", "Per million usage rows", "Storage and processing of cost records.", 2, 0, 100000, "USD", ""),
		floatDef(KeyCostCalcRows, "costs", "Per thousand carbon calculation rows", "Storage and processing of carbon calculation rows.", 0.05, 0, 100000, "USD", ""),
		floatDef(KeyCostSyncRun, "costs", "Per synchronization run", "Calls to the customer's cloud APIs and the processing around them.", 0.01, 0, 1000, "USD", ""),
		floatDef(KeyCostDeliveries, "costs", "Per thousand notification deliveries", "E-mail, webhook and chat delivery.", 0.1, 0, 100000, "USD", ""),
		floatDef(KeyCostReport, "costs", "Per generated report", "Rendering and storing a report file.", 0.02, 0, 1000, "USD", ""),
		floatDef(KeyCostK8sSamples, "costs", "Per million Kubernetes samples", "Storage and processing of workload energy samples.", 3, 0, 100000, "USD", ""),

		// ---- cost reporting (organization) ----
		{Key: KeyReportingCurrency, Group: "reporting", Title: "Reporting currency", Description: "The currency every cost figure is shown in. Amounts in other currencies are converted at the day's ECB reference rate. \"auto\" uses the currency most of your cost is billed in.",
			Scope: ScopeTenant, Type: Enum, Default: "auto",
			Enum: []string{"auto", "USD", "EUR", "GBP", "CHF", "JPY", "CAD", "AUD", "SEK", "NOK", "DKK", "PLN", "CZK", "HUF", "INR", "BRL", "SGD"},
			Note: "Reference rates for reporting, not for invoicing. An amount in a currency without a published rate is left out of the totals and the overview says so."},

		// ---- languages (platform) ----
		{Key: KeyDefaultLocale, Group: "i18n", Title: "Default language", Description: "Used for people who have not chosen a language and whose browser does not ask for an offered one; also for e-mails and reports without a recipient preference.",
			Scope: ScopePlatform, Type: Enum, Default: "en", Enum: []string{"en", "ru"}},
		{Key: KeyEnabledLocales, Group: "i18n", Title: "Offered languages", Description: "Languages people can choose in the language switcher.",
			Scope: ScopePlatform, Type: List, Default: []string{"en", "ru"}, ListEnum: []string{"en", "ru"}, MinItems: 1, MaxItems: 2,
			Note: "A language needs a complete translation catalog in the build; the list can only contain languages that exist."},

		// ---- grid data sources (platform) ----
		{Key: KeyCarbonPriority, Group: "carbon", Title: "Source priority", Description: "Sources of grid carbon intensity, most trusted first. For a day, the first source with enough readings is used; sources are never averaged together.",
			Scope: ScopePlatform, Type: List, Default: []string{"electricitymaps", "uk_carbon_intensity", "eia", "energy_charts", "ember"}, ListEnum: []string{"electricitymaps", "ember", "uk_carbon_intensity", "eia", "energy_charts"}, MinItems: 1, MaxItems: 7,
			Note: "Only sources that have a released connector can supply data. Marginal intensity (WattTime) is a different measure and is not part of this list."},
		{Key: KeyCarbonOverrides, Group: "carbon", Title: "Source per region", Description: "Use a specific source for a region, before the priority list. One entry per line as region=source, for example eu-west-2=uk_carbon_intensity.",
			Scope: ScopePlatform, Type: List, Default: []string{}, MaxItems: 100, MaxLen: 80, Pattern: `^[a-z0-9-]{2,40}=(electricitymaps|ember|uk_carbon_intensity|eia|energy_charts)$`},
		intDef(KeyCarbonMaxRequests, "carbon", "Requests per day to the grid data provider", "Cost control: once this many requests were made today to the provider (all processes together), no more are made until the next UTC day, and stored readings are used. 0 is unlimited.", ScopePlatform, 2000, 0, 1000000, "requests", "The hourly refresh needs about one request per region in use; loading past data needs one per region per 9 days."),
		boolDef(KeyCarbonAutoLoad, "carbon", "Load the past of a region by itself", "Every day, a region in use whose last days are not covered by grid data gets its past loaded from every source that can supply it, and the affected calculations are redone. Requests count against the daily cap.", ScopePlatform, true, "Turn off to load history only by hand from this page."),
		intDef(KeyCarbonAutoDays, "carbon", "How far back to load by itself", "The window the automatic loading covers and recalculates.", ScopePlatform, 90, 7, 365, "days", ""),
		intDef(KeyCarbonMinReadings, "carbon", "Readings a source needs per day", "A source's mean counts as the day's intensity only with at least this many readings that day; fewer is used only when no source has enough and is marked partial.", ScopePlatform, 6, 1, 24, "readings", "Readings are hourly."),
		intDef(KeyCarbonMaxAgeHours, "carbon", "Oldest reading used for a day without data", "For a day with no readings, the last reading before it is used if it is not older than this; an older one is used only as a marked estimate.", ScopePlatform, 72, 1, 720, "hours", ""),

		// ---- security (organization) ----
		intDef(KeyInviteExpiryDays, "security", "Invitation lifetime", "How long an invitation can be accepted.", ScopeTenant, 7, 1, 30, "days", "Applies to invitations created after the change."),
		{Key: KeyInviteAllowedDomain, Group: "security", Title: "Invitation e-mail domains", Description: "Only addresses in these domains can be invited. Empty allows every domain.",
			Scope: ScopeTenant, Type: List, Default: []string{}, MaxItems: 50, MaxLen: 100, Pattern: domainRe},
		intDef(KeyAPIKeysMaxActive, "security", "Active API keys", "Maximum number of API keys that are not revoked.", ScopeTenant, 10, 1, 50, "", ""),

		// ---- alerts (organization) ----
		intDef(KeyAlertsWarnPct, "alerts", "Warning level", "A budget or carbon limit raises a warning at this share of the limit; reaching the limit itself is critical.", ScopeTenant, 80, 10, 99, "%", "Applies at the next hourly check."),
		intDef(KeyAlertsCarbonMonthlyKg, "alerts", "Monthly carbon limit", "Alert when the organization's emissions in a calendar month approach this many kg CO2e. 0 turns the carbon alert off.", ScopeTenant, 0, 0, 1000000000, "kg CO2e", "Operational emissions, as shown on Carbon & Energy."),

		// ---- cloud connectors ----
		{Key: KeyProvidersAllowed, Group: "connectors", Title: "Allowed providers", Description: "Cloud providers this organization may connect.", Scope: ScopeOperator, Type: List,
			Default: []string{"aws", "gcp"}, ListEnum: []string{"aws", "gcp", "azure"}, MinItems: 1, MaxItems: 3, Note: "Azure is available when the platform has its Microsoft Entra application configured; listing it before that has no effect."},
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
		intDef(KeyDataRetentionMonths, "retention", "Usage and carbon history", "Monthly usage and carbon records older than this many months are dropped, for every organization. 0 keeps everything.", ScopePlatform, 0, 0, 120, "months", "Values from 1 to 12 are treated as 13: period comparisons need a full year of history. Whole months are dropped, never the current one."),
		intDef(KeyDeletionGraceDays, "retention", "Organization erasure grace period", "Days between an owner requesting the erasure of an organization and the erasure itself. The owner can cancel in that time.", ScopePlatform, 14, 1, 90, "days", ""),
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

// KeyMaxRequestsPerDay is the setting that caps a paid provider's daily requests (only the grid provider has one so far).
func KeyMaxRequestsPerDay(provider string) string {
	_ = provider
	return KeyCarbonMaxRequests
}
