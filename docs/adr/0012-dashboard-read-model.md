# ADR-0012 Read-model modules may read other schemas

Status: accepted

Decision: dashboard (Overview screen), reports (datasets) and recommendations (generator inputs) are consumers: their repositories read usage, carbon and audit tables read-only, avoiding N round trips and cross-domain calls inside a monolith. They own no foreign tables and never write them. If usage or carbon is extracted, these move to API composition or materialized views.
