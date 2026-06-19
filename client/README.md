# email-service client (Go)

Producer-side client for the Pennsieve email-service. Import it from any Go
service that needs to send a platform email.

```
go get github.com/pennsieve/email-service/client
```

It builds an `EmailRequest` and puts it on the email-service SQS queue. The
email-service consumer Lambda renders the template (with org branding), delivers
via SES, journals the send, and handles idempotency. **Producers never touch
SES.**

## Usage

```go
import (
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	emailclient "github.com/pennsieve/email-service/client"
)

cfg, _ := config.LoadDefaultConfig(ctx)
c := emailclient.New(sqs.NewFromConfig(cfg), queueURL) // queueURL from the queue's Terraform output

req := emailclient.DatasetPublicationAccepted(
	emailclient.To{Name: "Alice", Email: "alice@example.com"},
	emailclient.DatasetPublicationAcceptedArgs{
		DatasetName:  "My Dataset",
		ReviewerName: "Bob",
		Date:         "2026-06-18",
	},
).WithOrganization(367) // optional: use the org's branded template

if err := c.Send(ctx, req); err != nil {
	// handle enqueue error
}
```

## API

- `New(sqsClient, queueURL) *Client` — construct the client.
- `(*Client).Send(ctx, EmailRequest) error` — validate + enqueue.
- **Typed builders** — one per template (e.g. `DatasetPublicationAccepted`,
  `ChangeOfDatasetOwner`, `AddedToTeam`, `RehydrationComplete`, …) — each takes
  a `To` and an `Args` struct and returns a validated `EmailRequest`. The builder
  owns the `messageId` and the context keys the template expects.
- `Message(messageId, to, context)` — untyped escape hatch.
- Chainable options on `EmailRequest`: `WithOrganization(id)`,
  `WithDedupeId(id)`, `WithSubject(s)`.

## Builders are generated

`messages.go` is **generated** from the template manifest
(`contract/template-variables.json`) — do not hand-edit it. After a template's
variables change (and the manifest is updated), regenerate both the Go and Scala
builders:

```bash
make generate   # = go run internal/gen/main.go
```

This guarantees the builders never drift from the templates' actual variables.

## Notes

- `Send` is asynchronous: it returns once the message is on the queue; delivery
  happens later in the consumer.
- `dedupeId` (via `WithDedupeId`) is recommended when you have a stable id for a
  logical send; otherwise the consumer dedupes on a content hash.
