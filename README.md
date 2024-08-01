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
- **Partition Key**: `messageId`
- **Sort Key**: *TBD*

### Search Indexes
none

## Table: `email-message-log`
This table is a record of email messages sent by the **Email Service**

### Item Attributes
- `Id` : UUID
- `Timestamp`: Int64
- `MessageSent`: String(`timestamp`)
- `Recipient`: String(email address)
- `MessageId`: String
- `Context`: Map of String (name -> value)

### Keys
- **Partition Key**: `Id`
- **Sort Key**: `MessageId`

### Search Indexes
- **RecipientMessageIdIndex**: `Recipient` + `MessageId`
