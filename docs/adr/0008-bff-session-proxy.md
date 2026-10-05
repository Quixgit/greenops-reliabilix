# ADR-0008 Next.js as session holder and API proxy

Status: accepted

Decision: the browser talks only to Next.js. The Auth0 session is an httpOnly cookie; /api/proxy attaches the access token server-side, validates nothing itself (the Go API is the authority) and exposes only GET/POST under /api/v1. Responses are validated with zod. Dev login (DEV_AUTH) requires APP_ENV=dev and is useless against a non-dev API.
