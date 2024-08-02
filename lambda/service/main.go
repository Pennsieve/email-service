package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pennsieve/email-service/service/handler"
)

func main() {
	lambda.Start(handler.EmailServiceHandler)
}
