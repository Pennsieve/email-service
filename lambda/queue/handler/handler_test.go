package handler

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestQueueHandler(t *testing.T) {
	err := EmailQueueHandler(context.Background(), events.SQSEvent{
		Records: make([]events.SQSMessage, 0)})
	assert.NoError(t, err)
}
