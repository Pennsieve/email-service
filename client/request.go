// Package client is the producer-side client for the Pennsieve email-service.
//
// Other services use it to request an email by enqueuing an EmailRequest on the
// email-service SQS queue. They never call SES directly — the email-service
// consumer Lambda owns template rendering, SES delivery, journaling, and
// idempotency. This package only constructs the request and puts it on the
// queue.
//
// Typical use:
//
//	c := client.New(sqs.NewFromConfig(cfg), queueURL)
//	req := client.DatasetAccepted(
//		client.To{Name: "Alice", Email: "alice@example.com"},
//		client.DatasetAcceptedArgs{DatasetName: "My Dataset", ReviewerName: "Bob"},
//	).WithOrganization(367)
//	err := c.Send(ctx, req)
package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// EmailRequest is the JSON payload carried in the body of an SQS message. It is
// the wire contract between producers (this package) and the email-service
// consumer: which email to send (MessageId), who to send it to (Recipients),
// and the name-value pairs applied to the template (Context).
//
// DedupeId is optional. When a producer supplies one, it is used (combined with
// the recipient) as the idempotency key so a redelivered SQS message does not
// cause a second send. When absent, a deterministic hash of the request is used
// instead (see DedupeKey).
type EmailRequest struct {
	MessageId  string         `json:"messageId"`
	DedupeId   string         `json:"dedupeId,omitempty"`
	Recipients []Recipient    `json:"recipients"`
	Context    map[string]any `json:"context"`
}

// Recipient identifies a single destination for an email.
type Recipient struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// To is an alias used by the builders for readability at call sites.
type To = Recipient

// WithOrganization sets the organization id in the context so the consumer can
// resolve an org-branded template (custom/O{id}/) with fallback to the default.
// Returns the request for chaining.
func (r EmailRequest) WithOrganization(organizationId int64) EmailRequest {
	if r.Context == nil {
		r.Context = map[string]any{}
	}
	r.Context["organizationId"] = organizationId
	return r
}

// WithDedupeId sets an explicit idempotency id. Use it when the producer can
// supply a stable id for a logical send (preferred over relying on the content
// hash). Returns the request for chaining.
func (r EmailRequest) WithDedupeId(dedupeId string) EmailRequest {
	r.DedupeId = dedupeId
	return r
}

// WithSubject overrides the template's default subject for this send. Returns
// the request for chaining.
func (r EmailRequest) WithSubject(subject string) EmailRequest {
	if r.Context == nil {
		r.Context = map[string]any{}
	}
	r.Context["subject"] = subject
	return r
}

// DedupeKey returns the idempotency key for sending this request to the given
// recipient. If the producer supplied a DedupeId, the key is DedupeId+recipient
// so the same logical send is deduped per recipient. Otherwise the key is a
// SHA-256 over (messageId, recipient, canonicalized context) so that an
// identical redelivered message maps to the same key without any producer
// cooperation. The consumer uses this for its idempotency guard.
func (r EmailRequest) DedupeKey(recipientEmail string) string {
	if r.DedupeId != "" {
		return r.DedupeId + ":" + recipientEmail
	}
	h := sha256.New()
	h.Write([]byte(r.MessageId))
	h.Write([]byte{0})
	h.Write([]byte(recipientEmail))
	h.Write([]byte{0})
	h.Write(canonicalContext(r.Context))
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalContext returns a stable byte representation of the context map so
// the fallback dedupe hash is order-independent. Keys are sorted and each value
// is JSON-encoded.
func canonicalContext(ctx map[string]any) []byte {
	if len(ctx) == 0 {
		return nil
	}
	keys := make([]string, 0, len(ctx))
	for k := range ctx {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf []byte
	for _, k := range keys {
		buf = append(buf, k...)
		buf = append(buf, '=')
		// json.Marshal of a value that came from json.Unmarshal cannot fail;
		// ignore the error defensively.
		v, _ := json.Marshal(ctx[k])
		buf = append(buf, v...)
		buf = append(buf, ';')
	}
	return buf
}

// OrganizationId returns the organization id from the request context, if one
// is present. The second return value is false when the context has no
// organizationId, in which case the consumer uses the default (unbranded)
// template. JSON numbers unmarshal into float64, so we accept that and a few
// other representations to be forgiving of producers.
func (r EmailRequest) OrganizationId() (int64, bool) {
	v, ok := r.Context["organizationId"]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// Subject returns the subject line to use for the email. A subject in the
// request context overrides the default subject from the template mapping.
func (r EmailRequest) Subject(defaultSubject string) string {
	if v, ok := r.Context["subject"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return defaultSubject
}
