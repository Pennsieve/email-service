package handler

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/email-service/service/logging"
	"log/slog"
)

var logger = logging.Default

func init() {
	logger.Info("init()")
}

func EmailServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (*events.APIGatewayV2HTTPResponse, error) {
	logger = logger.With(slog.String("requestID", request.RequestContext.RequestID))

	apiResponse, err := handleRequest()

	return apiResponse, err
}

func handleRequest() (*events.APIGatewayV2HTTPResponse, error) {
	logger.Info("handleRequest()")
	apiResponse := events.APIGatewayV2HTTPResponse{Body: "{'response':'hello'}", StatusCode: 200}

	return &apiResponse, nil
}
