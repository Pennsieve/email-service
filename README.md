# email-service
A Serverless Service that sends emails for the Pennsieve platform

The **Email Service** receives requests from an SQS queue and via an API endpoint. The requests specify which email is to be sent, and to whom it should be sent. The service uses DynamoDB tables to store service-specific metadata and a record of email messages sent.

# Entry points

## SQS Queue
`{env}-email-service-send-message-queue-use1`

## API endpoint
**POST** `/email/send`

- internal use only
- requires Service Token

## Request Format

The *send email* request is a JSON object with three top-level elements:

- `messageId` is a slug-style identifier of the email to be sent
- `recipients` is a list of objects with `name` and `email` attributes, which identify where the message will be sent
- `context` is the collection of values to be replaced in the email template

Example:
```json
{
  "messageId": string,
  "recipients": [
    {
      "name": string,
      "email": string
    }
  ],
  "context": {
    "organizationId": number,
    "organizationNodeId": string,
    "organiztionName": string,
    "userId": number,
    "userNodeId": number,
    "userName": string,
    "datasetId": number,
    "datasetNodeId": number,
    "datasetName": string,
    "customMessage": string,
    "field1": value,
    "field2": value
  }
}
```

# Internals

## Environment Variables

| Variable | Description |
|----------|-------------|
| PENNSIEVE_DOMAIN | the DNS domain in which the service is running |
| S3_BUCKET | the name of the AWS S3 bucket where the email templates are stored |
| TEMPLATE_FOLDER | the name of the "folder" in the AWS S3 bucket where the email templates are stored |

## Table: `email-message-templates`
This table maps the `messageId` to a file object on AWS S3. It also contains the default *subject* line for the email message. The default *subject* may be overridden if there is a `subject` in the message `context`.

### Item Attributes
- `MessageId`: String, slug-style 
- `Subject`: String, the default subject line for the email message
- `TemplateFile`: String, the name of the template file

### Keys
- **Partition Key**: `MessageId`

### Search Indexes
none

## Table: `email-message-log`
This table is the **journal** of email messages handled by the **Email Service**: one row per (message, recipient). It serves three purposes — an audit record, the idempotency guard against SQS redelivery, and operational troubleshooting ("I never got the email"). Rows expire via a DynamoDB TTL after `JOURNAL_TTL_DAYS` (default 90).

### Item Attributes
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

### Keys
- **Partition Key**: `Id`

### Search Indexes
- **RecipientSentAtIndex**: `Recipient` (HASH) + `SentAtKey` (RANGE) — find all emails to a recipient, newest-first (query with `ScanIndexForward=false`). `MessageId` is projected (`ALL`) so it can be filtered client-side or with a `FilterExpression`.

# Troubleshooting

To investigate whether an email was sent to a user, query the journal by recipient with `scripts/email-log.sh`:

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

Read the `Status` on the returned row(s): no row means the send was never queued; `FAILED` shows the `Error`; `SENT` includes the `SesMessageId` to trace delivery in SES.
