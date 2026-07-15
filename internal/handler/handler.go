package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-go/events"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/pennsieve/email-service/client"
	emailconfig "github.com/pennsieve/email-service/internal/config"
	"github.com/pennsieve/email-service/internal/journal"
	"github.com/pennsieve/email-service/internal/logging"
	"github.com/pennsieve/email-service/internal/mailer"
	"github.com/pennsieve/email-service/internal/store"
)

var logger = logging.Default

// now is a seam that tests can override for deterministic timestamps.
var now = func() time.Time { return time.Now().UTC() }

func init() {
	logger.Info("init()")
}

// EmailQueueHandler is the Lambda entry point. It processes each SQS record
// independently and returns the set of records that failed so that, with
// ReportBatchItemFailures enabled on the event source mapping, only failed
// messages are retried (and eventually sent to the DLQ).
func EmailQueueHandler(ctx context.Context, sqsEvent events.SQSEvent) (events.SQSEventResponse, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		// Without AWS config we cannot process any record; fail the whole batch.
		return events.SQSEventResponse{}, fmt.Errorf("error loading AWS config: %w", err)
	}
	cfg := emailconfig.NewConfig(awsCfg, emailconfig.LoadEnv())
	return ProcessEvent(ctx, cfg, sqsEvent), nil
}

// ProcessEvent processes every record in the event using the given config and
// returns the batch item failures. It is exported (and takes an explicit
// config) so tests can drive it with mocked collaborators.
func ProcessEvent(ctx context.Context, cfg *emailconfig.Config, sqsEvent events.SQSEvent) events.SQSEventResponse {
	// Log the batch size up front. A count of 0 means the invocation carried no
	// SQS records — e.g. a Lambda "Test" payload that wasn't wrapped in the SQS
	// event envelope ({"Records":[{"body":"..."}]}) — which otherwise looks like
	// a silent success (no journal write, no send, no error).
	logger.Info("processing SQS event", slog.Int("recordCount", len(sqsEvent.Records)))

	var response events.SQSEventResponse
	for _, message := range sqsEvent.Records {
		recordLogger := logger.With(slog.String("sqsMessageId", message.MessageId))
		if err := handleRecord(ctx, cfg, recordLogger, message); err != nil {
			recordLogger.Error("failed to process message", slog.Any("error", err))
			response.BatchItemFailures = append(response.BatchItemFailures,
				events.SQSBatchItemFailure{ItemIdentifier: message.MessageId})
		}
	}
	return response
}

func handleRecord(ctx context.Context, cfg *emailconfig.Config, log *slog.Logger, message events.SQSMessage) error {
	var request client.EmailRequest
	if err := json.Unmarshal([]byte(message.Body), &request); err != nil {
		return fmt.Errorf("error unmarshalling email request: %w", err)
	}
	if request.MessageId == "" {
		return fmt.Errorf("email request is missing messageId")
	}
	if len(request.Recipients) == 0 {
		return fmt.Errorf("email request %s has no recipients", request.MessageId)
	}
	log = log.With(slog.String("messageId", request.MessageId))
	log.Info("parsed email request", slog.Int("recipientCount", len(request.Recipients)))

	// 1. messageId -> template file + default subject
	mapping, err := cfg.TemplateStore().GetMapping(ctx, request.MessageId)
	if err != nil {
		return err
	}
	log.Info("resolved template mapping",
		slog.String("templateFile", mapping.TemplateFile),
		slog.String("defaultSubject", mapping.Subject))

	// 2. resolve organization branding and fetch the template body
	orgId, hasOrg := request.OrganizationId()
	body, err := cfg.BodyStore().FetchTemplate(ctx, orgId, hasOrg, mapping.TemplateFile)
	if err != nil {
		return err
	}
	log.Info("fetched template body",
		slog.Bool("orgBrandingRequested", hasOrg),
		slog.Int64("organizationId", orgId),
		slog.Int("bytes", len(body)))

	// 3. render once; the same rendered body is sent to every recipient
	htmlBody, err := store.Render(request.MessageId, body, request.Context)
	if err != nil {
		return err
	}

	// Resolve the subject (context override > table default) then render it
	// against the context so default subjects can use {{.var}} interpolation.
	subject, err := store.RenderSubject(request.MessageId, request.Subject(mapping.Subject), request.Context)
	if err != nil {
		return err
	}
	log.Info("rendered template", slog.String("subject", subject), slog.Int("htmlBytes", len(htmlBody)))

	// Send controls (message-level): the whole record is log-only if the service
	// send switch is off, or this template has sending disabled. Per-recipient
	// address suppression is checked in sendOne. In all cases the request is
	// still journaled (see LoggedOnly on the entry).
	messageLogOnly := ""
	if !cfg.Env.SendEnabled {
		messageLogOnly = "service send disabled"
	} else if mapping.SendDisabled {
		messageLogOnly = "template send disabled"
	}
	if messageLogOnly != "" {
		log.Info("message is log-only", slog.String("reason", messageLogOnly))
	}

	// 4. for each recipient: claim (idempotency guard) -> send (or log-only) -> mark.
	for _, recipient := range request.Recipients {
		if recipient.Email == "" {
			return fmt.Errorf("messageId %s has a recipient with no email address", request.MessageId)
		}
		if err := sendOne(ctx, cfg, log, request, subject, htmlBody, recipient.Email, messageLogOnly); err != nil {
			return err
		}
	}

	return nil
}

// sendOne processes one (message, recipient). messageLogOnly is non-empty when
// a message-level control (service/template) has already forced log-only; sendOne
// additionally checks per-recipient address suppression. When any control
// applies, the request is journaled with LoggedOnly=true and SES is skipped.
func sendOne(ctx context.Context, cfg *emailconfig.Config, log *slog.Logger, request client.EmailRequest, subject, htmlBody, recipientEmail, messageLogOnly string) error {
	dedupeKey := request.DedupeKey(recipientEmail)
	sentAt := now()

	// Address-level control: is this recipient on the suppression list?
	logOnlyReason := messageLogOnly
	if logOnlyReason == "" {
		suppressed, err := cfg.Suppression().IsSuppressed(ctx, recipientEmail)
		if err != nil {
			return err
		}
		if suppressed {
			logOnlyReason = "recipient suppressed"
		}
	}
	logOnly := logOnlyReason != ""

	log.Info("claiming send",
		slog.String("recipient", recipientEmail),
		slog.String("dedupeKey", dedupeKey),
		slog.Bool("logOnly", logOnly))

	// Claim is a conditional write: if a row for this dedupe key already exists,
	// this (message, recipient) was already processed on an earlier delivery, so
	// we skip to avoid a double-send. The QUEUED row is also the journal entry.
	err := cfg.Journal().Claim(ctx, journal.Entry{
		Id:         dedupeKey,
		MessageId:  request.MessageId,
		Recipient:  recipientEmail,
		Status:     journal.StatusQueued,
		Timestamp:  sentAt.Unix(),
		SentAtKey:  journal.SentAtKey(sentAt),
		Context:    request.Context,
		ExpiresAt:  sentAt.AddDate(0, 0, cfg.Env.JournalTTLDays).Unix(),
		LoggedOnly: logOnly,
	})
	if errors.Is(err, journal.ErrAlreadyClaimed) {
		log.Info("skipping duplicate send", slog.String("recipient", recipientEmail))
		return nil
	}
	if err != nil {
		return err
	}

	// Log-only: the request is journaled (claimed above, marked below) but not
	// delivered. Every request is recorded regardless of the send controls.
	if logOnly {
		if err := cfg.Journal().MarkLoggedOnly(ctx, dedupeKey, sentAt.Format(time.RFC3339)); err != nil {
			return err
		}
		log.Info("logged only (not sent)",
			slog.String("recipient", recipientEmail),
			slog.String("reason", logOnlyReason))
		return nil
	}

	sesMessageId, sendErr := cfg.Mailer().Send(ctx, mailer.Email{
		Recipient: recipientEmail,
		Subject:   subject,
		HTMLBody:  htmlBody,
	})
	if sendErr != nil {
		// Mark the row FAILED, then surface the error so the SQS message is
		// retried (and eventually DLQ'd). Because Claim accepts a FAILED row,
		// the redelivery re-claims and retries this recipient; the FAILED
		// status also makes the failure visible when answering "I never got the
		// email" if it ends up in the DLQ.
		if markErr := cfg.Journal().MarkFailed(ctx, dedupeKey, sendErr.Error()); markErr != nil {
			log.Error("failed to mark journal entry failed", slog.Any("error", markErr))
		}
		return sendErr
	}

	if err := cfg.Journal().MarkSent(ctx, dedupeKey, sesMessageId, sentAt.Format(time.RFC3339)); err != nil {
		return err
	}
	log.Info("email sent", slog.String("recipient", recipientEmail), slog.String("sesMessageId", sesMessageId))
	return nil
}
