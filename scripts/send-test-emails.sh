#!/usr/bin/env bash
#
# send-test-emails.sh — drive test sends for every email template and verify each
# landed in the journal. Builds an EmailRequest per messageId from the template
# manifest (contract/template-variables.json), filling each variable with a
# plausible sample value derived from its name (overridable via
# testdata/sample-values.json), enqueues to the email-service SQS queue, then
# polls email-message-log for the result.
#
# Usage:
#   ENV=dev ./scripts/send-test-emails.sh --dry-run           # print+validate, no AWS
#   ENV=dev ./scripts/send-test-emails.sh                     # send all + verify
#   ENV=dev ./scripts/send-test-emails.sh dataset-proposal-submitted rehydration-complete
#   ENV=dev QUEUE_URL=... ./scripts/send-test-emails.sh --fresh   # unique dedupeId per run
#
# Options / env:
#   --dry-run          print each payload and validate its keys; do not send
#   --fresh            append a timestamp to dedupeId so a re-run actually resends
#   --recipient <addr> override the test recipient (default verify@example.com)
#   ENV                dev|prod — used to resolve the queue URL and journal table
#   QUEUE_URL          override the send-queue URL (else derived from ENV)
#   REGION_SHORTNAME   region suffix in resource names (default use1)
#
# Requires: awscli v2 (for non-dry-run), jq.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="$ROOT/contract/template-variables.json"
OVERRIDES="$ROOT/testdata/sample-values.json"
REGION_SHORTNAME="${REGION_SHORTNAME:-use1}"
RECIPIENT="verify@example.com"
DRY_RUN=0
FRESH=0

die() { echo "error: $*" >&2; exit 1; }
command -v jq >/dev/null || die "jq is required"
[[ -f "$MANIFEST" ]] || die "manifest not found: $MANIFEST"

# --- parse args -------------------------------------------------------------
FILTER=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --fresh) FRESH=1; shift ;;
    --recipient) RECIPIENT="${2:?}"; shift 2 ;;
    -h|--help) sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) die "unknown option: $1" ;;
    *) FILTER+=("$1"); shift ;;
  esac
done

# messageIds to process: the filter args, or all from the manifest.
# (Read into an array without mapfile, which is absent in Bash 3.2 / macOS.)
MESSAGE_IDS=()
if [[ ${#FILTER[@]} -gt 0 ]]; then
  MESSAGE_IDS=("${FILTER[@]}")
else
  while IFS= read -r line; do
    [[ -n "$line" ]] && MESSAGE_IDS+=("$line")
  done < <(jq -r 'keys[]' "$MANIFEST")
fi

# suffix for dedupeId when --fresh (forces a resend past the idempotency guard)
FRESH_SUFFIX=""
[[ "$FRESH" -eq 1 ]] && FRESH_SUFFIX="-$(date +%s)"

# --- sample value for a variable, by name -----------------------------------
# Precedence handled by the caller (override → global → this default).
default_value() {
  local key="$1"
  local lc; lc="$(echo "$key" | tr '[:upper:]' '[:lower:]')"
  case "$lc" in
    organizationid)                 echo "367" ;;
    *email*)                        echo "sample.person@example.com" ;;
    *nodeid)                        echo "N:organization:00000000-0000-0000-0000-000000000000" ;;
    host|discoverhost)              echo "app.pennsieve.net" ;;
    *url)                           echo "app.pennsieve.net" ;;
    date|*date)                     echo "2026-07-15" ;;
    *link)                          echo "https://app.pennsieve.net/setup" ;;
    datasetid|datasetversionid|discoverdatasetid) echo "12345" ;;
    awsregion)                      echo "us-east-1" ;;
    transactionnumber|requestid)    echo "abc123-request-id" ;;
    *)                              echo "Sample ${key}" ;;
  esac
}

# override lookup: testdata/sample-values.json  [<messageId> then _global]
override_value() {
  local mid="$1" key="$2"
  [[ -f "$OVERRIDES" ]] || return 1
  local v
  v="$(jq -r --arg m "$mid" --arg k "$key" '(.[$m][$k]) // empty' "$OVERRIDES")"
  [[ -n "$v" ]] && { echo "$v"; return 0; }
  v="$(jq -r --arg k "$key" '(._global[$k]) // empty' "$OVERRIDES")"
  [[ -n "$v" ]] && { echo "$v"; return 0; }
  return 1
}

# build the context object for a messageId
build_context() {
  local mid="$1"
  local keys; keys="$(jq -r --arg m "$mid" '.[$m] // empty | .[]' "$MANIFEST")"
  [[ -n "$keys" ]] || die "unknown messageId '$mid' (not in manifest)"
  local obj="{}"
  while IFS= read -r key; do
    [[ -n "$key" ]] || continue
    local val
    val="$(override_value "$mid" "$key" || default_value "$key")"
    # organizationId is numeric in the contract; everything else is a string.
    if [[ "$key" == "organizationId" ]]; then
      obj="$(echo "$obj" | jq -c --arg k "$key" --argjson v "$val" '. + {($k): $v}')"
    else
      obj="$(echo "$obj" | jq -c --arg k "$key" --arg v "$val" '. + {($k): $v}')"
    fi
  done <<< "$keys"
  # ensure branding is exercised even if the template doesn't list organizationId
  echo "$obj" | jq -c 'if has("organizationId") then . else . + {organizationId: 367} end'
}

# build the full EmailRequest JSON for a messageId
build_request() {
  local mid="$1"
  local ctx; ctx="$(build_context "$mid")"
  jq -n --arg mid "$mid" --arg dedupe "verify-${mid}${FRESH_SUFFIX}" \
        --arg rcpt "$RECIPIENT" --argjson ctx "$ctx" '{
    messageId: $mid,
    dedupeId: $dedupe,
    recipients: [ { name: "Verify Recipient", email: $rcpt } ],
    context: $ctx
  }'
}

# --- queue + journal resolution (mirrors email-log.sh conventions) ----------
queue_url() {
  if [[ -n "${QUEUE_URL:-}" ]]; then echo "$QUEUE_URL"; return; fi
  [[ -n "${ENV:-}" ]] || die "set ENV=dev|prod or QUEUE_URL"
  # Resolve the queue URL by name from SQS.
  local name="${ENV}-email-service-queue-${REGION_SHORTNAME}"
  aws sqs get-queue-url --queue-name "$name" --query QueueUrl --output text
}

journal_table() {
  [[ -n "${ENV:-}" ]] || die "set ENV=dev|prod"
  echo "${ENV}-email-message-log-${REGION_SHORTNAME}"
}

# --- dry run ----------------------------------------------------------------
if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "DRY RUN — building ${#MESSAGE_IDS[@]} payload(s), not sending."
  fail=0
  for mid in "${MESSAGE_IDS[@]}"; do
    req="$(build_request "$mid")"
    # validate: payload context keys == manifest keys (minus injected organizationId)
    want="$(jq -r --arg m "$mid" '.[$m][]' "$MANIFEST" | sort | tr '\n' ',')"
    got="$(echo "$req" | jq -r '.context | keys[]' | grep -v '^organizationId$' | sort | tr '\n' ',')"
    if [[ "$got" != "$want" ]]; then
      echo "  ✗ $mid: context keys mismatch (got [$got] want [$want])"; fail=1
    else
      echo "  ✓ $mid"
    fi
    echo "$req" | jq .
  done
  [[ "$fail" -eq 0 ]] || die "one or more payloads failed key validation"
  exit 0
fi

# --- send + verify ----------------------------------------------------------
command -v aws >/dev/null || die "awscli is required to send"
QURL="$(queue_url)"
TABLE="$(journal_table)"
echo "sending ${#MESSAGE_IDS[@]} message(s) to $QURL"

declare -a SENT_IDS=()
for mid in "${MESSAGE_IDS[@]}"; do
  req="$(build_request "$mid")"
  aws sqs send-message --queue-url "$QURL" --message-body "$req" >/dev/null
  echo "  sent $mid"
  SENT_IDS+=("verify-${mid}${FRESH_SUFFIX}:${RECIPIENT}")
done

echo "waiting for the consumer to process..."
sleep 10

echo "=== journal results ($TABLE) ==="
ok=0; bad=0
for id in "${SENT_IDS[@]}"; do
  item="$(aws dynamodb get-item --table-name "$TABLE" \
            --key "{\"Id\":{\"S\":\"$id\"}}" --consistent-read \
            --query 'Item.Status.S' --output text 2>/dev/null || echo "None")"
  case "$item" in
    SENT)   echo "  ✓ SENT    $id"; ok=$((ok+1)) ;;
    FAILED) echo "  ✗ FAILED  $id"; bad=$((bad+1)) ;;
    *)      echo "  ? MISSING $id (no journal row yet)"; bad=$((bad+1)) ;;
  esac
done
echo "=== $ok sent, $bad not-sent of ${#SENT_IDS[@]} ==="
[[ "$bad" -eq 0 ]]
