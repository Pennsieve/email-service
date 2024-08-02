package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pennsieve/email-service/queue/handler"
)

func main() {
	lambda.Start(handler.EmailQueueHandler)
}
