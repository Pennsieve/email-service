# email-service client (Scala)

Producer-side client for the Pennsieve email-service, for Scala services
(notably pennsieve-api, which sends the majority of platform emails).

It builds an `EmailRequest` and puts it on the email-service SQS queue; the
consumer Lambda renders the template (with org branding), delivers via SES,
journals the send, and handles idempotency. **Producers never touch SES.**

The JSON wire contract is shared with the [Go client](../client) — both test
against the fixtures in [`../contract/fixtures`](../contract), so the two cannot
drift.

## Usage

```scala
import com.pennsieve.email.client.{EmailClient, Messages, Recipient}
import software.amazon.awssdk.services.sqs.SqsClient

val client = new EmailClient(SqsClient.create(), queueUrl) // queueUrl from the queue's Terraform output

val request = Messages
  .datasetPublicationAccepted(
    Recipient("Alice", "alice@example.com"),
    datasetName = "My Dataset",
    reviewerName = "Bob",
    date = "2026-06-18"
  )
  .withOrganization(367L) // optional: use the org's branded template

client.send(request) match {
  case scala.util.Success(sqsMessageId) => // enqueued
  case scala.util.Failure(err)          => // handle validation / enqueue error
}
```

## API

- `new EmailClient(sqs, queueUrl)` — construct the client.
- `client.send(request): Try[String]` — validate + enqueue; returns the SQS
  message id.
- **Typed builders** in `Messages` (e.g. `datasetPublicationAccepted`,
  `changeOfDatasetOwner`, `addedToTeam`, `rehydrationComplete`,
  `datasetProposalSubmitted`) — each returns an `EmailRequest`. The builder owns
  the `messageId` and the context keys the template expects.
- `Messages.message(messageId, to, context)` — untyped escape hatch.
- Chainable on `EmailRequest`: `withOrganization(id)`, `withDedupeId(id)`,
  `withSubject(s)`.

## Building

```bash
sbt test      # compiles + runs the contract tests
sbt publish   # publish the artifact (configure the repo in build.sbt)
```

## Keeping in sync with the Go client

When the wire contract changes, update the fixtures in `../contract/fixtures`
and both clients in the same PR — the contract tests on each side will fail
until the JSON shapes match again. When adding a builder, use the exact context
keys from the email-templates repo's `template-variables.json`, matching the Go
client's builder.
