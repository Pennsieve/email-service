package main

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pennsieve/email-service/internal/bounce"
	"github.com/pennsieve/email-service/internal/suppression"
)

func main() {
	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		panic("error loading AWS config: " + err.Error())
	}
	store := suppression.NewDynamoStore(dynamodb.NewFromConfig(awsCfg), os.Getenv("SUPPRESSION_TABLE"))
	handler := bounce.NewHandler(store, func() string { return time.Now().UTC().Format(time.RFC3339) })
	lambda.Start(handler.Handle)
}
