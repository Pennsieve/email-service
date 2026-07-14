# Email test matrix

One SQS test event per template messageId (25 files), for verifying that the
email-service renders and delivers every email correctly. See
[`docs/email-inventory.md`](../../docs/email-inventory.md) for the full inventory.

Each `<messageId>.json` is a complete SQS event envelope whose record `body` is
an `EmailRequest` with:

- the correct `messageId`,
- a `dedupeId` of `verify-<messageId>`,
- one recipient (`verify@example.com` — change before sending for real),
- `context` containing **every key the template requires** (per the
  email-templates `template-variables.json`), each set to `SAMPLE_<key>`, plus
  `organizationId: 367` to exercise org-branding resolution.

The `SAMPLE_<key>` values make rendering gaps obvious: a correctly rendered
email shows e.g. `SAMPLE_datasetName` where the variable was; a blank or a
literal `{{.datasetName}}` means the substitution failed.

## Using a payload

- **Lambda Test console:** paste the file contents as the test event.
- **Real queue (end-to-end through the event source mapping):**

  ```bash
  aws sqs send-message \
    --queue-url "$EMAIL_SERVICE_QUEUE_URL" \
    --message-body "$(jq -r '.Records[0].body' dataset-proposal-submitted.json)"
  ```

Then confirm the journal row and delivery:

```bash
ENV=dev ../../scripts/email-log.sh id 'verify-dataset-proposal-submitted:verify@example.com'
```

## Notes

- Re-sending the same file is a no-op after the first success (the `dedupeId`
  makes it idempotent). Change the `dedupeId` to force a resend.
- `error-publishing` and `invite-external-new-user-to-dataset` have payloads
  here for completeness even though no service sends them today.
