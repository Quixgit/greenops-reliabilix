#!/usr/bin/env bash
# Deploys the current checkout to the demonstration stand (docs/setup/stand.md): builds the Go binaries and the
# frontend, then restarts the four user units. The stand serves the frontend from this checkout, so `next build`
# replaces the files under a running server; restarting right after the build is part of the deploy, never skip it
# (a server started before the build fails with ChunkLoadError on pages it had not rendered yet).
#
#   scripts/stand-deploy.sh            # build + restart
#   STAND_DB_CONTAINER=name STAND_DB=greenops_stand scripts/stand-deploy.sh migrate [0037 0038 ...]
#       applies the Up part of the migrations with these numbers by psql (default: only the newest file). The stand database
#       has no goose table, so the numbers to apply are the ones added since the last deploy.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
ver="$(tr -d '[:space:]' < "$root/VERSION")"
bin="$HOME/reliabilix-stand/bin"
pkg=github.com/quixgit/greenops-reliabilix/backend/internal/platform/version

if [ "${1:-}" = "migrate" ]; then
  : "${STAND_DB_CONTAINER:?set STAND_DB_CONTAINER}" "${STAND_DB:?set STAND_DB}"
  shift
  if [ "$#" -eq 0 ]; then files="$(ls "$root"/backend/migrations/*.sql | sort | tail -1)"; else
    files=""; for n in "$@"; do files="$files $(ls "$root"/backend/migrations/"$n"_*.sql)"; done; fi
  for f in $files; do
    echo "applying the Up part of $(basename "$f") to $STAND_DB"
    awk '/-- \+goose Down/{exit} {print}' "$f" | docker exec -i "$STAND_DB_CONTAINER" psql -U "${STAND_DB_USER:-postgres}" -d "$STAND_DB" -v ON_ERROR_STOP=1 -1
  done
  exit 0
fi

echo "building $ver"
mkdir -p "$bin"
for c in api worker scheduler; do
  (cd "$root/backend" && go build -ldflags "-X $pkg.Platform=$ver -X $pkg.Service=$c" -o "$bin/$c.new" "./cmd/$c")
done
(cd "$root/frontend" && NEXT_PUBLIC_BUILD_VERSION="$ver" npx next build >/dev/null)
for c in api worker scheduler; do mv "$bin/$c.new" "$bin/$c"; done
for n in api worker scheduler; do sed -i "s/^RELEASE=.*/RELEASE=$ver/" "$HOME/.config/reliabilix/$n.env"; done
systemctl --user restart reliabilix-api reliabilix-worker reliabilix-scheduler reliabilix-web
sleep 5
systemctl --user is-active reliabilix-api reliabilix-worker reliabilix-scheduler reliabilix-web
echo "deployed $ver"
