# ADR-0013 Recommendations: compliance first, human approval, no silent changes

Status: accepted

Decision: a recommendation carries carbon and cost impact separately (cost is null and basis not_estimated unless a region price index exists). region_shift is generated only into regions in the project's data-residency allow-list; with an empty list nothing is proposed. Status flows open -> approved -> applied (or dismissed) through explicit calls by people with recommendation:apply (never API keys); compliance is re-checked against the CURRENT policy at approve and apply; the row is locked so concurrent decisions serialize; every step is audited. Apply records the decision; automation (Terraform/Kubernetes execution with rollback) is phase 3 and will hang off the approved state.
