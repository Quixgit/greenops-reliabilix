# ADR-0009 Dashboard read-model module

Status: accepted

Decision: a read-only module composes usage, carbon and audit data for the Overview screen. It owns no tables and never writes. It deliberately reads other schemas to avoid N round-trips and cross-domain calls inside a monolith; if usage or carbon is extracted, dashboard moves to API composition or a materialized view.
