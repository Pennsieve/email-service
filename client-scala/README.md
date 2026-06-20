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

## Versioning & publishing

Releases are **intentional and tag-driven**. CI (the email-service `Jenkinsfile`)
publishes to Nexus as follows:

| Trigger | Publishes | Where |
|---|---|---|
| **git tag `vX.Y.Z`** (or a GitHub Release from it) | release `X.Y.Z` | `maven-releases` |
| **merge to `main`** | next-minor `-SNAPSHOT` (latest `v1.2.0` → `1.3.0-SNAPSHOT`) | `maven-snapshots` |
| other branches | nothing (test only) | — |

Consumers pin a released semver (e.g. `com.pennsieve %% email-client-scala % "1.2.0"`),
never a SNAPSHOT.

**To cut a release:** tag the commit `vX.Y.Z` and push the tag (or publish a
GitHub Release); CI publishes `X.Y.Z`. Use `v`-prefixed tags to match the house
convention (e.g. pennsieve-go-core); the artifact version drops the `v`.

```bash
# local builds (version comes from -Dversion; bare publish is a dev SNAPSHOT)
sbt test                       # compile + run the contract tests
sbt -Dversion=1.2.3 publish    # publish 1.2.3 (needs Nexus creds in env)
```

Publishing requires `PENNSIEVE_NEXUS_USER` / `PENNSIEVE_NEXUS_PW` in the
environment (the Jenkins executor provides them via the `pennsieve-nexus-ci-login`
credential).

## Keeping in sync with the Go client

When the wire contract changes, update the fixtures in `../contract/fixtures`
and both clients in the same PR — the contract tests on each side will fail
until the JSON shapes match again. Builders are generated from the template
manifest, so adding/changing a template's variables is: update
`contract/template-variables.json`, then `make generate` (regenerates both the
Go and Scala builders).
