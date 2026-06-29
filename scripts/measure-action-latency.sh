#!/usr/bin/env bash
# Mesure latence cloud_actions : inject → delivered → done (armoire seule, poll SC944D).
#
# Local:
#   ./scripts/measure-action-latency.sh --machine-id 15 --limit 15
#
# OVH:
#   MEASURE_ACTION_SSH=ubuntu@test.essensys.fr \
#     ./scripts/measure-action-latency.sh --email nicolas@rineau.eu --limit 20
#
set -euo pipefail

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-essensys}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_NAME="${DB_NAME:-essensys_db}"

FILTER_MACHINE_ID=""
FILTER_EMAIL=""
LIMIT=15

usage() {
  sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
  echo ""
  echo "Usage: $0 [--machine-id N] [--email EMAIL] [--limit N]"
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --machine-id) FILTER_MACHINE_ID="$2"; shift 2 ;;
    --email) FILTER_EMAIL="$2"; shift 2 ;;
    --limit) LIMIT="$2"; shift 2 ;;
    -h|--help) usage 0 ;;
    *) echo "Unknown arg: $1" >&2; usage 1 ;;
  esac
done

run_psql() {
  local sql="$1"
  local ssh_host="${MEASURE_ACTION_SSH:-${VERIFY_INVENTORY_SSH:-}}"
  if [[ -n "$ssh_host" ]]; then
    ssh -o ConnectTimeout=15 "$ssh_host" \
      "sudo -u postgres psql ${DB_NAME} -v ON_ERROR_STOP=1 -c $(printf '%q' "$sql")"
  else
    export PGPASSWORD="$DB_PASSWORD"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
      -v ON_ERROR_STOP=1 -c "$sql"
  fi
}

where_clause() {
  local parts=()
  [[ -n "$FILTER_MACHINE_ID" ]] && parts+=("ca.machine_id = ${FILTER_MACHINE_ID}")
  if [[ -n "$FILTER_EMAIL" ]]; then
    parts+=("ca.user_id = (SELECT id FROM users WHERE email = '${FILTER_EMAIL}' LIMIT 1)")
  fi
  if [[ ${#parts[@]} -eq 0 ]]; then
    echo "TRUE"
  else
    local IFS=" AND "
    echo "${parts[*]}"
  fi
}

WHERE="$(where_clause)"

SQL="
SELECT
  ca.guid,
  ca.status,
  ca.created_at AT TIME ZONE 'UTC' AS created_utc,
  ROUND(EXTRACT(EPOCH FROM (ca.delivered_at - ca.created_at))::numeric, 2) AS inject_to_delivered_s,
  ROUND(EXTRACT(EPOCH FROM (ca.done_at - ca.created_at))::numeric, 2) AS inject_to_done_s,
  ca.params::text AS params
FROM cloud_actions ca
WHERE ${WHERE}
ORDER BY ca.created_at DESC
LIMIT ${LIMIT};
"

echo "# cloud_actions latency (newest first)"
echo "# filter machine_id=${FILTER_MACHINE_ID:-*} email=${FILTER_EMAIL:-*} limit=${LIMIT}"
run_psql "$SQL"

SQL_STATS="
SELECT
  COUNT(*) AS n,
  ROUND(AVG(EXTRACT(EPOCH FROM (delivered_at - created_at)))::numeric, 2) AS avg_delivered_s,
  ROUND(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (delivered_at - created_at)))::numeric, 2) AS p50_delivered_s,
  ROUND(PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (delivered_at - created_at)))::numeric, 2) AS p90_delivered_s,
  ROUND(AVG(EXTRACT(EPOCH FROM (done_at - created_at)))::numeric, 2) AS avg_done_s,
  ROUND(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (done_at - created_at)))::numeric, 2) AS p50_done_s,
  ROUND(PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (done_at - created_at)))::numeric, 2) AS p90_done_s
FROM cloud_actions ca
WHERE ${WHERE}
  AND delivered_at IS NOT NULL
  AND created_at > NOW() - INTERVAL '7 days';
"

echo ""
echo "# stats 7 derniers jours (actions avec delivered_at)"
run_psql "$SQL_STATS"
