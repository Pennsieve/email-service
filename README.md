# email-service
A Serverless Service that sends emails for the Pennsieve platform

## Function
The **Email Service** receives messages from an SQS queue. The messages specify which email is to be sent, and to whom it should be sent. The service uses DynamoDB tables to store metadata (location of email templates on AWS S3) and a record of email messages sent.

## Environment

| Variable | Description |
|----------|-------------|
| PENNSIEVE_DOMAIN | the DNS domain in which the service is running |
| S3_BUCKET | the name of the AWS S3 bucket where the email templates are stored |
| TEMPLATE_FOLDER | the name of the "folder" in the AWS S3 bucket where the email templates are stored |

## Table: `email-message-templates`
This table maps the `messageId` to a file object on AWS S3. It also contains the default *subject* line for the email message. The default *subject* may be overridden if there is a `subject` in the message `context`.

### Item Attributes
- `messageId`: String, slug-style 
- `subject`: String, the default subject line for the email message
- `templateFile`: String, the name of the template file

### Keys
- **Partition Key**: `messageId`
- **Sort Key**: *TBD*

### Search Indexes
none

## Table: `email-message-log`
This table is a record of email messages sent by the **Email Service**

### Item Attributes
- `id` : UUID
- `timestamp`: Int64
- `messageSent`: String(`timestamp`)
- `recipient`: String(email address)
- `messageId`: String
- `context`: Map of String (name -> value)

### Keys
- **Partition Key**: `id`
- **Sort Key**: `messageId`

### Search Indexes
- **RecipientIndex**: `recipient` + `messageId`

## Request Format

The *send email* request is a JSON object with three top-level elements:

- `messageId` is a slug-style identifier of the email to be sent
- `context` is the collection of values to be replaced in the email template
- `recipients` is a list of names & email address to which the message will be sent

Example:
```json
{
  "messageId": string,
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
  },
  "recipients": [
    {
      "name": string,
      "email": string
    },
    {
      "name": string,
      "email": string
    },
  ],
}
```
