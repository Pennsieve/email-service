package handler

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/email-service/queue/logging"
	"log/slog"
)

var logger = logging.Default

func init() {
	logger.Info("init()")
}

func EmailQueueHandler(ctx context.Context, sqsEvent events.SQSEvent) error {
	for _, message := range sqsEvent.Records {
		logger = logger.With(slog.String("messageId", message.MessageId))
		handleRequest(message)
	}

	return nil
}

func handleRequest(message events.SQSMessage) error {
	logger.Info("handleRequest()")
	logger.Info(fmt.Sprintf("handleRequest() message: %+v", message))

	return nil
}
