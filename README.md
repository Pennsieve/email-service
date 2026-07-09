# email-service

A serverless service that sends emails for the Pennsieve platform.

Producers (other Pennsieve services) request an email by putting a message on an
SQS queue. They specify **which** email (a slug-style `messageId`), **who** it
goes to, and the **values** to fill into the template. A Lambda consumer renders
the template and delivers via SES. Producers never call SES themselves —
templating, branding, delivery, idempotency, and the audit trail all live here.

## How it works

```
producer (Go/Scala client)                 email-service                          AWS
──────────────────────────                  ─────────────                          ───
EmailRequest{messageId, recipients, context}
        │  enqueue (SQS SendMessage)
        ▼
   ┌─────────────┐   SQS    ┌────────────────────────────┐
   │ send queue  │ ───────▶ │ queue lambda (per record):  │
   └─────────────┘          │                             │
                            │ 1. look up messageId  ──────┼──▶ DynamoDB email-message-templates
                            │    → template file + subject │     (messageId → file, default subject)
                            │ 2. fetch template      ──────┼──▶ S3  custom/O{orgId}/{file}
                            │    (org branding, else       │        → fallback default/{file}
                            │     default)                 │
                            │ 3. render body + subject     │     (Go html/template; subject text/template)
                            │ 4. per recipient:            │
                            │    claim → send → mark  ─────┼──▶ DynamoDB email-message-log (journal)
                            │                              │──▶ SES SendEmail
                            └──────────────┬───────────────┘
                                           │ failures (after retries)
                                           ▼
                                    ┌─────────────┐
                                    │ dead-letter │
                                    └─────────────┘
```

Per SQS record the consumer:

1. **Resolves the template** — looks up `messageId` in the `email-message-templates`
   table to get the template file name and the default subject.
2. **Fetches the template from S3** — tries the org-branded path
   `custom/O{organizationId}/{file}` first (when `context.organizationId` is
   set), falling back to `default/{file}`.
3. **Renders** the body (Go `html/template`, auto-escaped) and the subject
   (`text/template`; a `context.subject` overrides the default).
4. **Sends and journals, per recipient** — a conditional write *claims* the
   send (the idempotency guard), then SES delivers, then the row is marked
   `SENT`/`FAILED`.

**Idempotency & retries.** Each (message, recipient) has a dedupe key. The claim
is a conditional `PutItem`: a duplicate SQS redelivery of an already-`SENT`/
`QUEUED` row is skipped (no double-send), while a `FAILED` row is allowed to be
retried. The consumer reports per-record batch-item failures, so one bad message
doesn't fail a whole batch; messages that keep failing land in the dead-letter
queue.

Templates live in a separate repo — see [Templates](#templates) below.

## Using the clients

Producers should use a client rather than build the SQS message by hand — the
clients own the queue wiring, JSON contract, dedupe id, and typed per-message
builders. Both clients produce the **same** wire payload (enforced by shared
fixtures in [`contract/`](contract)).

### Go — [`client/`](client)

```go
import (
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	emailclient "github.com/pennsieve/email-service/client"
)

cfg, _ := config.LoadDefaultConfig(ctx)
c := emailclient.New(sqs.NewFromConfig(cfg), queueURL) // EMAIL_SERVICE_QUEUE_URL

req := emailclient.DatasetPublicationAccepted(
	emailclient.To{Name: "Alice", Email: "alice@example.com"},
	emailclient.DatasetPublicationAcceptedArgs{
		DatasetName:  "My Dataset",
		ReviewerName: "Bob",
		Date:         "2026-06-22",
	},
).WithOrganization(367) // optional: use the org's branded template

if err := c.Send(ctx, req); err != nil { /* handle enqueue error */ }
```

See [`client/README.md`](client/README.md) for the full API. Builders are
generated from the template manifest (`make generate`).

### Scala — [`client-scala/`](client-scala)

```scala
import com.pennsieve.email.client.{EmailClient, Messages, Recipient}
import software.amazon.awssdk.services.sqs.SqsClient

val client = new EmailClient(SqsClient.create(), queueUrl) // EMAIL_SERVICE_QUEUE_URL

val request = Messages
  .datasetPublicationAccepted(
    Recipient("Alice", "alice@example.com"),
    datasetName = "My Dataset",
    reviewerName = "Bob",
    date = "2026-06-22"
  )
  .withOrganization(367L)

client.send(request) // returns Try[String]
```

Published to the Pennsieve Nexus as `com.pennsieve %% email-client-scala`
(release versions are cut from `vX.Y.Z` git tags). See
[`client-scala/README.md`](client-scala/README.md).

### The request contract

Every send is an `EmailRequest`:

```jsonc
{
  "messageId": "dataset-publication-accepted",   // which template
  "dedupeId": "optional-stable-id",               // optional idempotency id
  "recipients": [{ "name": "Alice", "email": "alice@example.com" }],
  "context": {                                    // template variables + reserved keys
    "organizationId": 367,                        // optional → branded template
    "subject": "override",                        // optional → overrides default subject
    "datasetName": "My Dataset", "reviewerName": "Bob", "date": "2026-06-22"
  }
}
```

The `context` keys a template needs are listed per `messageId` in the
email-templates repo's `template-variables.json` (the typed builders encode them
for you).

## Templates

Email templates are **not** in this repo — they live in
**[Pennsieve/email-templates](https://github.com/Pennsieve/email-templates)**.
Keeping them separate lets template copy/branding change and deploy on its own
cadence, without touching or redeploying this service.

That repo holds the MJML source for each email (~25 templates), shared
`header`/`footer` partials, and per-organization branding under
`mjml/custom/O{orgId}/`. Its CI compiles MJML → HTML and `aws s3 sync`s the
output into this service's `S3_BUCKET` (`default/` for the standard templates,
`custom/O{orgId}/` for branded overrides) — which is exactly the layout step 2
of [How it works](#how-it-works) reads from.

Two files in email-templates are the contract between the repos:

- **`template-mapping-seed.json`** — seeds the `email-message-templates`
  DynamoDB table (`messageId` → template file + default subject), so a new
  template can be wired up without a code change here.
- **`template-variables.json`** — the per-`messageId` list of `context` keys
  each template expects. It is the source of truth the client builders are
  generated from (`make generate`), so the producers, the templates, and the
  rendered output stay in lockstep.

**Adding or changing an email** is therefore mostly an email-templates change
(add/edit the MJML, update the two manifests); the only change here is
regenerating the client builders from the updated `template-variables.json`.

## Internals

### Entry point

SQS queue: `{env}-email-service-queue-use1` (consumed by the queue lambda via an
event source mapping with `ReportBatchItemFailures`).

### Environment variables

| Variable | Description |
|----------|-------------|
| `PENNSIEVE_DOMAIN` | DNS domain the service runs in; sets the sender `Pennsieve <support@{domain}>` and `Reply-To: support@{domain}` |
| `S3_BUCKET` | S3 bucket holding the compiled email templates |
| `TEMPLATES_TABLE` | DynamoDB table mapping `messageId` → template file + subject |
| `JOURNAL_TABLE` | DynamoDB table journaling sent messages (`email-message-log`) |
| `JOURNAL_TTL_DAYS` | retention for journal rows before TTL expiry (default 90) |

### Table: `email-message-templates`
Maps `messageId` to a template file in S3 and the default *subject* line. The
default subject may be overridden by a `subject` in the message `context`.

#### Item Attributes
- `MessageId`: String, slug-style
- `Subject`: String, the default subject line for the email message
- `TemplateFile`: String, the name of the template file

#### Keys
- **Partition Key**: `MessageId`

#### Search Indexes
none

### Table: `email-message-log`
The **journal** of email messages handled by the service: one row per (message,
recipient). It serves three purposes — an audit record, the idempotency guard
against SQS redelivery, and operational troubleshooting ("I never got the
email"). Rows expire via a DynamoDB TTL after `JOURNAL_TTL_DAYS` (default 90).

#### Item Attributes
- `Id`: String, the idempotency / dedupe key — `{dedupeId}:{recipient}` when the producer supplies a `dedupeId`, otherwise a SHA-256 of `messageId` + `recipient` + canonicalized `context`
- `MessageId`: String, slug-style id of the email
- `Recipient`: String (email address)
- `Status`: String — `QUEUED` (claimed), `SENT` (accepted by SES), or `FAILED`
- `Timestamp`: Int64 (Unix epoch seconds)
- `SentAtKey`: String, zero-padded Unix epoch — the GSI sort key (lexicographic order == time order)
- `MessageSent`: String (RFC3339 timestamp), set on `SENT`
- `SesMessageId`: String, the SES message id, set on `SENT`
- `Error`: String, the failure detail, set on `FAILED`
- `Context`: Map of String (name -> value)
- `ExpiresAt`: Int64 (Unix epoch seconds), DynamoDB TTL attribute

#### Keys
- **Partition Key**: `Id`

#### Search Indexes
- **RecipientSentAtIndex**: `Recipient` (HASH) + `SentAtKey` (RANGE) — find all emails to a recipient, newest-first (query with `ScanIndexForward=false`). `MessageId` is projected (`ALL`) so it can be filtered client-side or with a `FilterExpression`.

## Troubleshooting

To investigate whether an email was sent to a user, query the journal by
recipient with `scripts/email-log.sh`:

```bash
# All emails to a recipient, most-recent first
ENV=dev ./scripts/email-log.sh recipient jane@example.com

# Only a specific email type
ENV=dev ./scripts/email-log.sh recipient jane@example.com --message-id welcome

# Just the latest email to a recipient
ENV=dev ./scripts/email-log.sh latest jane@example.com

# A single row by its Id (dedupe key)
ENV=dev ./scripts/email-log.sh id 'abc123:jane@example.com'
```

Read the `Status` on the returned row(s): no row means the send was never
queued; `FAILED` shows the `Error`; `SENT` includes the `SesMessageId` to trace
delivery in SES.

## Possible future additions

- **Synchronous API endpoint** (e.g. `POST /email/send`, service-token
  protected) for callers that need an immediate send rather than enqueuing. Not
  implemented today — the service is SQS-only. The original scaffold included an
  API lambda; it was removed in favor of the queue-only design and would be
  re-added here if a synchronous path is needed.
