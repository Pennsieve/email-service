package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pennsieve/email-service/internal/handler"
)

func main() {
	lambda.Start(handler.EmailQueueHandler)
}
