#!/usr/bin/env bash
# Smoke test for a running stack (docker compose up --build -d). Dev tokens only: refuses to run against a non-dev API.
# Usage: scripts/smoke.sh [api_url] [web_url]
set -euo pipefail
API="${1:-http://127.0.0.1:8080}"
WEB="${2:-http://127.0.0.1:3000}"
TENANT="00000000-0000-4000-8000-000000000001"   # created by the seed-dev service
TOKEN="dev:smoke:${TENANT}:owner"
fail() { echo "FAIL: $*" >&2; exit 1; }
check() { # name expected_status method path [auth]
  local name="$1" want="$2" method="$3" path="$4" auth="${5:-}" code
  if [ -n "$auth" ]; then code=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" -H "Authorization: Bearer $TOKEN" "$API$path")
  else code=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" "$API$path"); fi
  [ "$code" = "$want" ] || fail "$name: $method $path returned $code, want $want"
  echo "ok   $name"
}
for i in $(seq 1 30); do curl -sf "$API/healthz" >/dev/null && break; sleep 2; [ "$i" = 30 ] && fail "API did not become healthy"; done
check "liveness"                 200 GET  /healthz
check "readiness (db + redis)"   200 GET  /readyz
check "anonymous is rejected"    401 GET  /api/v1/projects
check "garbage token rejected"   401 GET  /api/v1/projects x
code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer garbage" "$API/api/v1/projects"); [ "$code" = 401 ] || fail "garbage token returned $code"
check "dev tenant sees projects" 200 GET  /api/v1/projects auth
check "methodology is public to members" 200 GET /api/v1/carbon/methodology auth
check "empty tenant summary"     200 GET  /api/v1/carbon/summary auth
body=$(curl -s -H "Authorization: Bearer $TOKEN" "$API/api/v1/carbon/summary")
echo "$body" | grep -q '"has_data":false' || echo "note: tenant already has data (summary: ${body:0:80}...)"
check "security headers present" 200 GET  /healthz
curl -sI "$API/healthz" | grep -qi '^x-content-type-options: nosniff' || fail "missing security headers"
curl -sI "$API/healthz" | grep -qi '^x-request-id:' || fail "missing request id"
code=$(curl -s -o /dev/null -w '%{http_code}' "$WEB/login"); { [ "$code" = 200 ] || [ "$code" = 307 ]; } || fail "web /login returned $code"
echo "ok   web reachable"
echo "SMOKE PASSED"
