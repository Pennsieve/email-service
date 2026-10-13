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

Published to Pennsieve's AWS CodeArtifact `maven` repository (pennsieve-cc). A
consumer needs that resolver and a CodeArtifact token; see `build.sbt` here for
the few lines that set both up from the active AWS credentials. (1.0.0 was
published to the old Nexus, before the move.)

## Versioning & publishing

Semver releases are **intentional and tag-driven**; every `main` build also
publishes a build version. CI (the email-service `Jenkinsfile`) publishes to
CodeArtifact as follows:

| Trigger | Publishes | Where |
|---|---|---|
| **git tag `vX.Y.Z`** (or a GitHub Release from it) | release `X.Y.Z` | `maven` |
| **merge to `main`** | `<build>-<commit>`, e.g. `31-3d9f63b` (same scheme as the other Pennsieve clients) | `maven` |
| other branches | nothing (test only) | — |

Consumers pin an exact version: a semver release (e.g. `com.pennsieve %% email-client-scala % "1.2.0"`)
or a main build. No snapshots are published, and a published version can't be replaced.

**To cut a release:** tag the commit `vX.Y.Z` and push the tag (or publish a
GitHub Release); CI publishes `X.Y.Z`. Use `v`-prefixed tags to match the house
convention (e.g. pennsieve-go-core); the artifact version drops the `v`.

```bash
# local builds (version comes from -Dversion; bare publish is a dev SNAPSHOT)
sbt test                       # compile + run the contract tests
sbt -Dversion=1.2.3 publish    # publish 1.2.3 (CI only; see below)
```

Access to CodeArtifact is IAM-based: sbt exchanges the active AWS credentials for
a short-lived token. Locally, `assume-role non-prod admin` lets you build and test;
publishing is only allowed for Jenkins (the build agents' instance role). A
release version can't be republished: CodeArtifact rejects an existing version.

## Keeping in sync with the Go client

When the wire contract changes, update the fixtures in `../contract/fixtures`
and both clients in the same PR — the contract tests on each side will fail
until the JSON shapes match again. Builders are generated from the template
manifest, so adding/changing a template's variables is: update
`contract/template-variables.json`, then `make generate` (regenerates both the
Go and Scala builders).
