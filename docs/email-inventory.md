# Email inventory

A verification reference for every platform email: which service produces it,
what template/context it uses, who receives it, and what triggers it. Use it to
confirm each producer and the email-service render and deliver correctly.

Derived from current code: the migrated `feature/email-service-producer` branches
for publishing-service and rehydration-service, and `main` for pennsieve-api
(not yet migrated — still sends via direct SES). Templates and default subjects
come from the email-templates repo (`template-mapping-seed.json`); the required
context keys per email come from its `template-variables.json`.

## How a send flows

```
producer → EmailRequest{messageId, recipients, context} → SQS → email-service
  → resolve messageId → template file + default subject (email-message-templates table)
  → fetch template from S3 (custom/O{orgId}/ then default/)
  → render body (html/template) + subject (text/template; context.subject overrides)
  → From: Pennsieve <support@{domain}>, Reply-To: support@{domain}  → SES
  → journal row (QUEUED → SENT/FAILED) in email-message-log
```

pennsieve-api does NOT yet use this flow — it renders `MessageTemplates` and
calls SES directly. Its rows below are what it sends today and what it will
enqueue once migrated.

## publishing-service — migrated (QueueNotifier)

| messageId | context keys | recipients | trigger |
|---|---|---|---|
| `dataset-proposal-submitted` | AppURL, AuthorEmail, AuthorName, ProposalTitle, WorkspaceName, WorkspaceNodeId | publishing team members | proposal DRAFT → SUBMITTED |
| `dataset-proposal-withdrawn` | AppURL, AuthorEmail, AuthorName, ProposalTitle, WorkspaceName, WorkspaceNodeId | publishing team members | SUBMITTED → WITHDRAWN |
| `dataset-proposal-accepted` | AppURL, ProposalTitle, WelcomeWorkspaceNodeId, WorkspaceName | proposal author | publisher accepts (dataset created) |
| `dataset-proposal-rejected` | AppURL, ProposalTitle, WelcomeWorkspaceNodeId, WorkspaceName | proposal author | publisher rejects |

## rehydration-service — migrated (QueueEmailer)

| messageId | context keys (source) | recipient | trigger |
|---|---|---|---|
| `rehydration-complete` | DatasetID, DatasetVersionID, RehydrationLocation, AWSRegion | requesting user | rehydration task succeeds |
| `rehydration-failed` | DatasetID, DatasetVersionID, RequestID, SupportEmailAddress | requesting user | rehydration task fails |

## pennsieve-api — NOT yet migrated (direct SES today)

18 send sites → 17 messageIds (`email-address-changed` fires twice: old + new address).

| messageId | recipients | trigger (site) |
|---|---|---|
| `notification-of-publication-to-contributor` | owner + contributors | published to Discover (DataSetPublishingHelper.scala:141) |
| `dataset-publication-accepted` | owner + contributors | accepted for publication (:182) |
| `dataset-embargo-accepted` | owner + contributors | embargoed (:223) |
| `embargo-dataset-release-accepted` | owner + contributors | embargo release approved (:261) |
| `embargoed-dataset-released` | owner + contributors | embargoed dataset released (:298) |
| `dataset-revision-needed` | owner + contributors | rejected, needs revision (:343) |
| `dataset-revision` | owner + contributors | revision accepted (:385) |
| `dataset-publication-in-review` | owner + contributors | publication requested (:428) |
| `dataset-submitted-for-review` | publishers | submitted to publishers (:474) |
| `embargo-access-requested` | dataset managers | preview access requested (:513) |
| `embargo-access-approved` | requesting user | access approved (:552) |
| `embargo-access-denied` | requesting user | access denied (:585) |
| `email-address-changed` | old email address | email change (UserController.scala:401) |
| `email-address-changed` | new email address | email change (UserController.scala:411) |
| `added-to-team` | added user | add user to team (OrganizationsController.scala:1310) |
| `invite-external-existing-user-to-dataset` | external user | invite existing external collaborator (DataSetsController.scala:2190) |
| `change-of-dataset-owner` | new owner | switch dataset owner (DataSetsController.scala:2626) |
| `added-to-organization` | added user | add/invite user to org (OrganizationManager.scala:995) |

Required context keys per messageId are in email-templates `template-variables.json`.

## Template coverage & gaps

23 of the 25 templates are actively produced. Two are **defined but never sent**:

- **`error-publishing`** — template + mapping exist, but no service sends it. A
  publish-failure path could/should, but none does today.
- **`invite-external-new-user-to-dataset`** — template exists; pennsieve-api's
  `inviteExternalNewUserToDataset` is defined but not wired to a send (the
  new-user path uses a custom welcome message). Dead/incomplete.

Out of scope: **Cognito emails** (`new-account-creation`, `password-reset`) are
sent by AWS Cognito directly, not through this system.

## Verifying a single email is sent correctly

For each messageId confirm end to end:

1. **Producer** builds the correct `messageId` and the context keys match
   `template-variables.json` (the typed client builders enforce this for the Go
   producers; pennsieve-api sets them by hand until migrated).
2. **Template resolves** — email-service finds the template file for the
   messageId; org branding falls back to `default/` when no `custom/O{id}/`.
3. **Renders fully** — no leftover `{{.var}}` placeholders in the delivered HTML
   (a missing context key renders empty — the common silent failure).
4. **Subject** is the default (from the mapping) unless `context.subject`
   overrides it.
5. **Envelope** — From `Pennsieve <support@{domain}>`, Reply-To `support@{domain}`.
6. **Journal** — an `email-message-log` row lands `SENT` with an SES message id
   (query with `scripts/email-log.sh`).

## Test matrix

`testdata/messages/<messageId>.json` holds one ready-to-send SQS test event per
template (25 total), each with every required context key set to a recognizable
`SAMPLE_<key>` value plus `organizationId` (to exercise branding resolution).
The context keys in each payload exactly match `template-variables.json`.

Paste a file into the Lambda **Test** console, or send its `body` to the queue.
Because values are `SAMPLE_<key>`, the rendered email visibly shows whether every
field was substituted (any literal `{{.` or blank means a rendering gap). See
[`testdata/messages/README.md`](../testdata/messages/README.md).

### Driver: send + verify all at once

[`scripts/send-test-emails.sh`](../scripts/send-test-emails.sh) builds a request
per messageId from the manifest (plausible sample values, overridable in
[`testdata/sample-values.json`](../testdata/sample-values.json)), sends to the
queue, and reports each result from the journal (SENT/FAILED/missing):

```bash
ENV=dev ./scripts/send-test-emails.sh --dry-run   # build + validate keys, no AWS
ENV=dev ./scripts/send-test-emails.sh             # send all + verify
ENV=dev ./scripts/send-test-emails.sh <messageId>...   # subset
ENV=dev ./scripts/send-test-emails.sh --fresh     # unique dedupeId to force resend
```
