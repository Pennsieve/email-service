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
- **Typed builders** in `Messages` — one per template (e.g.
  `datasetPublicationAccepted`, `changeOfDatasetOwner`, `addedToTeam`,
  `rehydrationComplete`, …) — each returns an `EmailRequest`. The builder owns
  the `messageId` and the context keys the template expects.
- `Messages.build(messageId, to, context)` — untyped escape hatch.
- Chainable on `EmailRequest`: `withOrganization(id)`, `withDedupeId(id)`,
  `withSubject(s)`.

`Messages.scala` is **generated** from `contract/template-variables.json`
(see the Go client's `make generate`) — do not hand-edit it.

## Artifact

```
com.pennsieve %% email-client-scala % <version>   // Scala 2.13
```

Published to the Pennsieve Nexus (`nexus.pennsieve.cc`), same as pennsieve-api,
so a consumer that already resolves Pennsieve artifacts needs no extra resolver.

## Building & publishing

```bash
sbt test                       # compile + run the contract tests
sbt publish                    # publish a -SNAPSHOT to Nexus
sbt -Dversion=1.2.3 publish    # publish release 1.2.3 to Nexus
```

Publishing requires `PENNSIEVE_NEXUS_USER` / `PENNSIEVE_NEXUS_PW` in the
environment (the Jenkins executor provides them). CI publishes from the
email-service `Jenkinsfile`: every `main` build publishes a SNAPSHOT; pass the
`RELEASE_VERSION` job parameter to cut a release.

## Keeping in sync with the Go client

When the wire contract changes, update the fixtures in `../contract/fixtures`
and both clients in the same PR — the contract tests on each side will fail
until the JSON shapes match again. Builders are generated from the template
manifest, so adding/changing a template's variables is: update
`contract/template-variables.json`, then `make generate` (regenerates both the
Go and Scala builders).
