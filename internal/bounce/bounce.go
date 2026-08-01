// Package bounce handles SES bounce and complaint notifications (delivered via
// SNS) by adding the affected addresses to the suppression list. This protects
// the SES account's reputation: continuing to send to hard-bouncing or
// complaining addresses is exactly what drives the bounce/complaint rates AWS
// suspends accounts over.
//
// Only PERMANENT bounces and ALL complaints suppress. Transient bounces
// (mailbox full, etc.) are ignored — suppressing them would permanently block a
// temporarily-unavailable address.
package bounce

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/email-service/internal/logging"
	"github.com/pennsieve/email-service/internal/suppression"
)

var logger = logging.Default

// sesNotification is the SES notification carried in the SNS message body.
type sesNotification struct {
	NotificationType string `json:"notificationType"`
	Bounce           struct {
		BounceType        string `json:"bounceType"` // Permanent | Transient | Undetermined
		BouncedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"bouncedRecipients"`
	} `json:"bounce"`
	Complaint struct {
		ComplainedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"complainedRecipients"`
	} `json:"complaint"`
}

// Handler builds an SNS handler that suppresses bounced/complained addresses.
type Handler struct {
	suppression suppression.Store
	// timestamp returns the CreatedAt to stamp on new suppression records.
	timestamp func() string
}

func NewHandler(store suppression.Store, timestamp func() string) *Handler {
	return &Handler{suppression: store, timestamp: timestamp}
}

// Handle processes an SNS event: for each record, parse the SES notification and
// suppress every address that permanently bounced or complained. Returning an
// error lets Lambda retry the whole event; a single malformed record is logged
// and skipped rather than blocking the batch.
func (h *Handler) Handle(ctx context.Context, event events.SNSEvent) error {
	for _, record := range event.Records {
		if err := h.handleMessage(ctx, record.SNS.Message); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) handleMessage(ctx context.Context, message string) error {
	var n sesNotification
	if err := json.Unmarshal([]byte(message), &n); err != nil {
		// Not a parseable SES notification — log and skip; retrying won't help.
		logger.Error("skipping unparseable SES notification", slog.Any("error", err))
		return nil
	}

	reason, addresses := classify(n)
	if reason == "" {
		logger.Info("SES notification requires no suppression",
			slog.String("notificationType", n.NotificationType),
			slog.String("bounceType", n.Bounce.BounceType))
		return nil
	}

	for _, addr := range addresses {
		if addr == "" {
			continue
		}
		if err := h.suppression.Suppress(ctx, suppression.Record{
			Email:     addr,
			Reason:    reason,
			CreatedAt: h.timestamp(),
		}); err != nil {
			// Surface the error so Lambda retries — we do want the suppression to
			// land eventually (a re-suppress is idempotent).
			return fmt.Errorf("suppressing %s (%s): %w", addr, reason, err)
		}
		logger.Info("suppressed address", slog.String("email", addr), slog.String("reason", reason))
	}
	return nil
}

// classify returns the suppression reason and affected addresses for a
// notification, or an empty reason when it should not suppress.
func classify(n sesNotification) (reason string, addresses []string) {
	switch n.NotificationType {
	case "Complaint":
		for _, r := range n.Complaint.ComplainedRecipients {
			addresses = append(addresses, r.EmailAddress)
		}
		return "complaint", addresses
	case "Bounce":
		// Only permanent (hard) bounces suppress; transient bounces are ignored.
		if n.Bounce.BounceType != "Permanent" {
			return "", nil
		}
		for _, r := range n.Bounce.BouncedRecipients {
			addresses = append(addresses, r.EmailAddress)
		}
		return "bounce", addresses
	default:
		return "", nil
	}
}
