#!/usr/bin/env bash
# Proves that a backup of the database can be restored, and measures how long that takes (docs/operations/backup-and-recovery.md).
#
# It dumps the database of a running container, restores the dump into a throwaway container of the same PostgreSQL major
# version, and compares the two: the row count of every table, and the number of RLS policies, indexes and constraints.
# Nothing in the source database is changed and the throwaway container is removed at the end.
#
#   SRC_CONTAINER=rlx-notif-test-pg SRC_DB=greenops_stand scripts/backup-restore-check.sh
#
# Optional: SRC_USER (default postgres), KEEP=1 (keep the dump file), OUT=dir (where the report and the dump go).
# A live source keeps taking writes while the dump runs, so a table passes when its restored count lies between its
# count before and after the dump; on a quiet database the three numbers are equal.
set -euo pipefail
: "${SRC_CONTAINER:?set SRC_CONTAINER}" "${SRC_DB:?set SRC_DB}"
user="${SRC_USER:-postgres}"
out="${OUT:-$(mktemp -d)}"; mkdir -p "$out"
tmp="rlx-restore-check-$$"
trap 'docker rm -f "$tmp" >/dev/null 2>&1 || true; [ -n "${KEEP:-}" ] || rm -f "$out/backup.dump" "$out/roles.sql"' EXIT

psql_src() { docker exec -i "$SRC_CONTAINER" psql -U "$user" -d "$SRC_DB" -Atq -v ON_ERROR_STOP=1 "$@"; }
psql_dst() { docker exec -i "$tmp" psql -U postgres -d restored -Atq -v ON_ERROR_STOP=1 "$@"; }

major="$(psql_src -c "show server_version_num" | cut -c1-2)"
image="postgres:$major"
echo "source: $SRC_CONTAINER/$SRC_DB (PostgreSQL $major), restoring into a throwaway $image"

counts() { # prints "schema.table<TAB>rows" for every ordinary and partitioned-leaf table of the application schemas
  local run="$1"
  local tables
  tables="$($run -c "select format('%I.%I', n.nspname, c.relname) from pg_class c join pg_namespace n on n.oid=c.relnamespace
                     where c.relkind in ('r') and n.nspname not in ('pg_catalog','information_schema') and n.nspname not like 'pg_toast%' order by 1")"
  local t
  while IFS= read -r t; do [ -z "$t" ] && continue; printf '%s\t%s\n' "$t" "$($run -c "select count(*) from $t" </dev/null)"; done <<<"$tables"
}
shape() { $1 -c "select 'policies', count(*) from pg_policies union all select 'indexes', count(*) from pg_indexes where schemaname not in ('pg_catalog','information_schema')
                 union all select 'constraints', count(*) from pg_constraint c join pg_namespace n on n.oid=c.connamespace where n.nspname not in ('pg_catalog','information_schema')
                 union all select 'tables', count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where c.relkind='r' and n.nspname not in ('pg_catalog','information_schema')"; }

counts psql_src >"$out/before.tsv"; shape psql_src >"$out/shape-src.tsv"

t0=$(date +%s)
docker exec "$SRC_CONTAINER" pg_dumpall -U "$user" --roles-only >"$out/roles.sql"
docker exec "$SRC_CONTAINER" pg_dump -U "$user" -Fc --no-owner "$SRC_DB" >"$out/backup.dump"
t1=$(date +%s)
size=$(stat -c %s "$out/backup.dump")

docker run -d --name "$tmp" -e POSTGRES_PASSWORD=restore-check "$image" >/dev/null
for _ in $(seq 1 60); do docker exec "$tmp" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
sleep 2
docker exec -i "$tmp" psql -U postgres -d postgres -q -v ON_ERROR_STOP=0 <"$out/roles.sql" >/dev/null 2>&1 || true  # roles that already exist are fine
docker exec "$tmp" createdb -U postgres restored
docker exec -i "$tmp" pg_restore -U postgres -d restored --no-owner --exit-on-error <"$out/backup.dump"
t2=$(date +%s)

counts psql_src >"$out/after.tsv"; counts psql_dst >"$out/restored.tsv"; shape psql_dst >"$out/shape-dst.tsv"

fail=0
while IFS=$'\t' read -r tbl rest; do
  b=$(awk -F'\t' -v t="$tbl" '$1==t{print $2}' "$out/before.tsv"); a=$(awk -F'\t' -v t="$tbl" '$1==t{print $2}' "$out/after.tsv")
  lo=$(( b < a ? b : a )); hi=$(( b > a ? b : a ))
  if [ "$rest" -lt "$lo" ] || [ "$rest" -gt "$hi" ]; then echo "MISMATCH $tbl: before=$b after=$a restored=$rest"; fail=1; fi
done <"$out/restored.tsv"
missing=$(comm -23 <(cut -f1 "$out/before.tsv" | sort) <(cut -f1 "$out/restored.tsv" | sort) || true)
[ -z "$missing" ] || { echo "MISSING tables after restore:"; echo "$missing"; fail=1; }
diff "$out/shape-src.tsv" "$out/shape-dst.tsv" >/dev/null || { echo "SHAPE differs (policies/indexes/constraints/tables):"; diff "$out/shape-src.tsv" "$out/shape-dst.tsv" || true; fail=1; }

rows=$(awk -F'\t' '{s+=$2} END{print s+0}' "$out/restored.tsv")
echo "tables: $(wc -l <"$out/restored.tsv"), rows: $rows, dump: $size bytes"
echo "dump took $((t1 - t0)) s, restore took $((t2 - t1)) s (container start included), total $((t2 - t0)) s"
if [ "$fail" -eq 0 ]; then echo "RESTORE CHECK PASSED"; else echo "RESTORE CHECK FAILED"; exit 1; fi
