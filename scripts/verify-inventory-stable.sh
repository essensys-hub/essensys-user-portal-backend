#!/usr/bin/env bash
# Snapshot / diff stable machine inventory (machines.id, hashed_pkey, user links).
# IP may change (DHCP) — id + hashed_pkey + linked_armoire_id must not.
#
# Local (DB_* env or defaults):
#   ./scripts/verify-inventory-stable.sh snapshot -o /tmp/inv-before.txt --ip 82.67.136.197
#   ansible … systemd restarted …
#   ./scripts/verify-inventory-stable.sh snapshot -o /tmp/inv-after.txt --ip 82.67.136.197
#   ./scripts/verify-inventory-stable.sh verify /tmp/inv-before.txt /tmp/inv-after.txt
#
# Remote OVH via SSH:
#   VERIFY_INVENTORY_SSH=ubuntu@test.essensys.fr \
#     ./scripts/verify-inventory-stable.sh snapshot -o /tmp/inv-before.txt --email nicolas@rineau.eu
#
set -euo pipefail

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-essensys}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_NAME="${DB_NAME:-essensys_db}"

FILTER_IP=""
FILTER_ID=""
FILTER_EMAIL=""
OUT_FILE=""
CMD="${1:-}"

usage() {
  sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
  echo ""
  echo "Usage:"
  echo "  $0 snapshot [-o FILE] [--ip IP] [--id N] [--email EMAIL]"
  echo "  $0 verify BEFORE AFTER"
  echo "  $0 check [--ip IP] [--id N] [--email EMAIL]   # print snapshot to stdout"
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    snapshot|verify|check) CMD="$1"; shift ;;
    -o) OUT_FILE="$2"; shift 2 ;;
    --ip) FILTER_IP="$2"; shift 2 ;;
    --id) FILTER_ID="$2"; shift 2 ;;
    --email) FILTER_EMAIL="$2"; shift 2 ;;
    -h|--help) usage 0 ;;
    *) break ;;
  esac
done

run_psql() {
  local sql="$1"
  if [[ -n "${VERIFY_INVENTORY_SSH:-}" ]]; then
    ssh -o ConnectTimeout=15 "$VERIFY_INVENTORY_SSH" \
      "sudo -u postgres psql ${DB_NAME} -tA -F'|' -v ON_ERROR_STOP=1 -c $(printf '%q' "$sql")"
  else
    export PGPASSWORD="$DB_PASSWORD"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
      -tA -F'|' -v ON_ERROR_STOP=1 -c "$sql"
  fi
}

machine_where() {
  local parts=()
  [[ -n "$FILTER_ID" ]] && parts+=("m.id = ${FILTER_ID}")
  [[ -n "$FILTER_IP" ]] && parts+=("m.ip_address = '${FILTER_IP}'")
  if [[ ${#parts[@]} -eq 0 ]]; then
    echo "1=1"
  else
    local IFS=" OR "
    echo "${parts[*]}"
  fi
}

user_where() {
  if [[ -n "$FILTER_EMAIL" ]]; then
    echo "u.email = '${FILTER_EMAIL}'"
  else
    echo "1=1"
  fi
}

collect_snapshot() {
  local ts
  ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  echo "# verify-inventory-stable snapshot ${ts}"
  echo "# stable: machines.id, hashed_pkey, client_id(portal), linked_* | IP informational only"
  echo "# filter ip=${FILTER_IP:-*} id=${FILTER_ID:-*} email=${FILTER_EMAIL:-*}"

  run_psql "
    SELECT 'MACHINE|' || m.id || '|' || m.hashed_pkey || '|' || COALESCE(m.client_id,'') || '|' ||
           COALESCE(m.ip_address,'') || '|' || m.is_active
    FROM machines m
    WHERE $(machine_where)
    ORDER BY m.id;
  "

  run_psql "
    SELECT 'USER|' || u.email || '|' || COALESCE(u.linked_machine_id::text,'') || '|' ||
           COALESCE(u.linked_armoire_id::text,'') || '|' || COALESCE(u.linked_gateway_id,'')
    FROM users u
    WHERE $(user_where)
    ORDER BY u.email;
  "

  run_psql "
    SELECT 'DUPLICATE_CLIENT_ID|' || client_id || '|' || COUNT(*)::text
    FROM machines
    WHERE client_id ~ '^[0-9]+\$'
    GROUP BY client_id
    HAVING COUNT(*) > 1;
  " || true
}

cmd_snapshot() {
  local data
  data="$(collect_snapshot)"
  if [[ -n "$OUT_FILE" ]]; then
    printf '%s\n' "$data" >"$OUT_FILE"
    echo "Wrote snapshot: $OUT_FILE"
  else
    printf '%s\n' "$data"
  fi
}

cmd_check() {
  collect_snapshot
}

stable_machine_line() {
  # MACHINE|id|hashed_pkey|client_id|ip|is_active -> id|hashed_pkey|client_id|is_active
  local line="$1"
  echo "$line" | awk -F'|' '{print $2"|"$3"|"$4"|"$6}'
}

stable_user_line() {
  local line="$1"
  echo "$line" | awk -F'|' '{print $2"|"$3"|"$4"|"$5}'
}

cmd_verify() {
  local before="${1:-}"
  local after="${2:-}"
  [[ -z "$before" || -z "$after" ]] && usage 1
  [[ ! -f "$before" ]] && { echo "Missing file: $before" >&2; exit 1; }
  [[ ! -f "$after" ]] && { echo "Missing file: $after" >&2; exit 1; }

  local failed=0

  while IFS= read -r b; do
    [[ "$b" =~ ^MACHINE\| ]] || continue
    local mid
    mid="$(echo "$b" | awk -F'|' '{print $2}')"
    local a
    a="$(grep -E "^MACHINE\\|${mid}\\|" "$after" | head -1 || true)"
    if [[ -z "$a" ]]; then
      echo "FAIL: machine vanished: $b" >&2
      failed=1
      continue
    fi
    local sb sa
    sb="$(stable_machine_line "$b")"
    sa="$(stable_machine_line "$a")"
    if [[ "$sb" != "$sa" ]]; then
      echo "FAIL: machine stable fields changed" >&2
      echo "  before: $sb" >&2
      echo "  after:  $sa" >&2
      failed=1
    fi
    local ip_b ip_a
    ip_b="$(echo "$b" | awk -F'|' '{print $5}')"
    ip_a="$(echo "$a" | awk -F'|' '{print $5}')"
    if [[ "$ip_b" != "$ip_a" ]]; then
      echo "WARN: IP changed (OK if DHCP): $ip_b -> $ip_a (id $(echo "$b" | awk -F'|' '{print $2}'))" >&2
    fi
  done <"$before"

  while IFS= read -r b; do
    [[ "$b" =~ ^USER\| ]] || continue
    local email
    email="$(echo "$b" | awk -F'|' '{print $2}')"
    local a
    a="$(grep -F "USER|${email}|" "$after" | head -1 || true)"
    if [[ -z "$a" ]]; then
      echo "FAIL: user vanished: $b" >&2
      failed=1
      continue
    fi
    local sb sa
    sb="$(stable_user_line "$b")"
    sa="$(stable_user_line "$a")"
    if [[ "$sb" != "$sa" ]]; then
      echo "FAIL: user links changed for $email" >&2
      echo "  before: $sb" >&2
      echo "  after:  $sa" >&2
      failed=1
    fi
  done <"$before"

  local dup
  dup="$(grep '^DUPLICATE_CLIENT_ID|' "$after" || true)"
  if [[ -n "$dup" ]]; then
    echo "FAIL: duplicate numeric client_id after deploy:" >&2
    echo "$dup" >&2
    failed=1
  fi

  if [[ "$failed" -eq 0 ]]; then
    echo "OK: stable inventory fields unchanged ($(basename "$before") -> $(basename "$after"))"
  else
    exit 1
  fi
}

case "$CMD" in
  snapshot) cmd_snapshot ;;
  check) cmd_check ;;
  verify) cmd_verify "${1:-}" "${2:-}" ;;
  *) usage 1 ;;
esac
