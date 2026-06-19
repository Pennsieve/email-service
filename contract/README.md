# Email request wire contract

The JSON shape exchanged between the email-service clients (producers) and the
email-service consumer Lambda. This is the **single source of truth** for the
contract; both the Go client (`client/`) and the Scala client (`client-scala/`)
and the consumer test against the fixtures here, so a change in any language
that breaks the shape fails CI.

## Shape

```jsonc
{
  "messageId": "string",          // required — slug identifying the template
  "dedupeId": "string",           // optional — explicit idempotency id
  "recipients": [                 // required, non-empty
    { "name": "string", "email": "string" }
  ],
  "context": {                    // template variables + reserved keys
    "organizationId": 367,        // optional — selects custom/O{id}/ branding
    "subject": "string",          // optional — overrides the default subject
    "...": "any"                  // template-specific variables
  }
}
```

- `messageId` maps to a template file via the `email-message-templates` table.
- `context` keys must match the template's variables (see the email-templates
  repo's `template-variables.json`).
- `organizationId` and `subject` in `context` are reserved/interpreted by the
  consumer; all other keys are passed to the template renderer.

## Fixtures

| File | Covers |
|---|---|
| `minimal.json` | required fields only, single recipient |
| `with-organization.json` | org branding via `context.organizationId` |
| `with-dedupe-id.json` | explicit `dedupeId` |
| `multi-recipient.json` | multiple recipients |

Field ordering in the fixtures is not significant — clients assert structural
(decoded) equality, not byte equality.
