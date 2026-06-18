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
	emailconfig "github.com/pennsieve/email-service/internal/config"
	"github.com/pennsieve/email-service/internal/journal"
	"github.com/pennsieve/email-service/internal/logging"
	"github.com/pennsieve/email-service/internal/mailer"
	"github.com/pennsieve/email-service/internal/models"
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
	var request models.EmailRequest
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

	// 1. messageId -> template file + default subject
	mapping, err := cfg.TemplateStore().GetMapping(ctx, request.MessageId)
	if err != nil {
		return err
	}

	// 2. resolve organization branding and fetch the template body
	orgId, hasOrg := request.OrganizationId()
	body, err := cfg.BodyStore().FetchTemplate(ctx, orgId, hasOrg, mapping.TemplateFile)
	if err != nil {
		return err
	}

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

	// 4. for each recipient: claim (idempotency guard) -> send -> mark.
	for _, recipient := range request.Recipients {
		if recipient.Email == "" {
			return fmt.Errorf("messageId %s has a recipient with no email address", request.MessageId)
		}
		if err := sendOne(ctx, cfg, log, request, subject, htmlBody, recipient.Email); err != nil {
			return err
		}
	}

	return nil
}

func sendOne(ctx context.Context, cfg *emailconfig.Config, log *slog.Logger, request models.EmailRequest, subject, htmlBody, recipientEmail string) error {
	dedupeKey := request.DedupeKey(recipientEmail)
	sentAt := now()

	// Claim is a conditional write: if a row for this dedupe key already exists,
	// this (message, recipient) was already processed on an earlier delivery, so
	// we skip to avoid a double-send. The QUEUED row is also the journal entry.
	err := cfg.Journal().Claim(ctx, journal.Entry{
		Id:        dedupeKey,
		MessageId: request.MessageId,
		Recipient: recipientEmail,
		Status:    journal.StatusQueued,
		Timestamp: sentAt.Unix(),
		SentAtKey: journal.SentAtKey(sentAt),
		Context:   request.Context,
		ExpiresAt: sentAt.AddDate(0, 0, cfg.Env.JournalTTLDays).Unix(),
	})
	if errors.Is(err, journal.ErrAlreadyClaimed) {
		log.Info("skipping duplicate send", slog.String("recipient", recipientEmail))
		return nil
	}
	if err != nil {
		return err
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
