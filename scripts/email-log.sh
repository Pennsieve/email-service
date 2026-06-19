#!/usr/bin/env bash
#
# email-log.sh — query the email-service message log (journal) in DynamoDB.
#
# The journal records one row per (email, recipient) with delivery Status
# (QUEUED/SENT/FAILED), the SES MessageId, and any error. Use it to answer
# "I never got the email": find the recipient's rows and inspect their status.
#
# Table:  {ENV}-email-message-log-{REGION_SHORT}   e.g. dev-email-message-log-use1
# Index:  RecipientSentAtIndex  (Recipient = HASH, SentAtKey = RANGE, newest-first)
#
# Requires: awscli v2, jq (optional — falls back to raw JSON if missing).
#
# Usage:
#   ENV=dev ./scripts/email-log.sh recipient <email> [--message-id <id>] [--limit N] [--oldest-first]
#   ENV=dev ./scripts/email-log.sh latest    <email>
#   ENV=dev ./scripts/email-log.sh id         <Id>
#
# Examples:
#   ENV=dev ./scripts/email-log.sh recipient jane@example.com
#   ENV=dev ./scripts/email-log.sh recipient jane@example.com --message-id welcome
#   ENV=dev ./scripts/email-log.sh latest jane@example.com
#   ENV=prod REGION_SHORT=use1 ./scripts/email-log.sh recipient bob@example.com --limit 5
#
# Environment:
#   ENV           required unless TABLE is set (e.g. dev, prod)
#   REGION_SHORT  region short name in the table suffix (default: use1)
#   TABLE         override the full table name (skips ENV/REGION_SHORT)
#   AWS_REGION / AWS_PROFILE  honored by the AWS CLI as usual

set -euo pipefail

REGION_SHORT="${REGION_SHORT:-use1}"
INDEX="RecipientSentAtIndex"

die() {
  echo "error: $*" >&2
  exit 1
}

usage() {
  sed -n '2,32p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

table_name() {
  if [[ -n "${TABLE:-}" ]]; then
    echo "$TABLE"
  elif [[ -n "${ENV:-}" ]]; then
    echo "${ENV}-email-message-log-${REGION_SHORT}"
  else
    die "set ENV (e.g. ENV=dev) or TABLE to identify the table"
  fi
}

# Pretty-print DynamoDB Items: flatten the {"S":..}/{"N":..} wrappers if jq is
# present; otherwise emit the raw AWS CLI JSON unchanged.
render() {
  if command -v jq >/dev/null 2>&1; then
    jq '[.Items[] | map_values(.S // .N // .BOOL // .NULL // .)] '\
'| sort_by(.Timestamp | tonumber)'
  else
    cat
  fi
}

# query_recipient <email> <scan_forward:true|false> <limit|""> <message_id|"">
query_recipient() {
  local email="$1" forward="$2" limit="$3" message_id="$4"
  local table
  table="$(table_name)"

  local -a args=(
    dynamodb query
    --table-name "$table"
    --index-name "$INDEX"
    --key-condition-expression "Recipient = :email"
    --scan-index-forward "$forward"
  )

  if [[ -n "$message_id" ]]; then
    args+=(
      --filter-expression "MessageId = :mid"
      --expression-attribute-values
      "{\":email\":{\"S\":\"$email\"},\":mid\":{\"S\":\"$message_id\"}}"
    )
  else
    args+=(
      --expression-attribute-values
      "{\":email\":{\"S\":\"$email\"}}"
    )
  fi

  [[ -n "$limit" ]] && args+=(--max-items "$limit")

  aws "${args[@]}" | render
}

cmd="${1:-}"
[[ -z "$cmd" || "$cmd" == "-h" || "$cmd" == "--help" ]] && usage 0
shift || true

case "$cmd" in
  recipient)
    [[ $# -ge 1 ]] || die "recipient requires an email address"
    email="$1"; shift
    limit=""; message_id=""; forward="false" # default newest-first
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --message-id) message_id="${2:-}"; shift 2 ;;
        --limit)      limit="${2:-}"; shift 2 ;;
        --oldest-first) forward="true"; shift ;;
        *) die "unknown option: $1" ;;
      esac
    done
    query_recipient "$email" "$forward" "$limit" "$message_id"
    ;;

  latest)
    [[ $# -eq 1 ]] || die "latest requires exactly one email address"
    # newest-first, single row
    query_recipient "$1" "false" "1" ""
    ;;

  id)
    [[ $# -eq 1 ]] || die "id requires exactly one Id value"
    table="$(table_name)"
    aws dynamodb get-item \
      --table-name "$table" \
      --key "{\"Id\":{\"S\":\"$1\"}}" \
      --consistent-read \
      | render
    ;;

  *)
    die "unknown command: $cmd (use recipient | latest | id; -h for help)"
    ;;
esac
