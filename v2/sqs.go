package formigo

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

var (
	ErrClientNil          = errors.New("formigo/v2: sqs client is nil")
	ErrInputNil           = errors.New("formigo/v2: sqs ReceiveMessage input is nil")
	ErrReceiptHandleEmpty = errors.New("formigo/v2: message ReceiptHandle is empty when deleting")
)

type SQSClient struct {
	client *sqs.Client
	input  *sqs.ReceiveMessageInput
}

func (s SQSClient) Fetch(ctx context.Context) ([]Message, error) {
	if s.client == nil {
		return nil, ErrClientNil
	}

	if s.input == nil {
		return nil, ErrInputNil
	}

	output, err := s.client.ReceiveMessage(ctx, s.input)
	if err != nil {
		return nil, err
	}

	return convert(output), nil
}

func (s SQSClient) Delete(ctx context.Context, msg Message) error {
	if msg.ReceiptHandle == "" {
		return ErrReceiptHandleEmpty
	}

	_, err := s.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      s.input.QueueUrl,
		ReceiptHandle: &msg.ReceiptHandle,
	})
	if err != nil {
		return err
	}

	return nil
}

func NewSQSClient(client *sqs.Client, input *sqs.ReceiveMessageInput) *SQSClient {
	return &SQSClient{
		client: client,
		input:  input,
	}
}

func convert(output *sqs.ReceiveMessageOutput) []Message {
	messages := make([]Message, 0, len(output.Messages))

	for _, msg := range output.Messages {
		if msg.MessageId == nil {
			// something horrible has happened
			continue
		}

		if msg.Body == nil {
			var empty string
			msg.Body = &empty
		}

		messages = append(messages, Message{
			ID:                *msg.MessageId,
			Body:              *msg.Body,
			ReceiptHandle:     *msg.ReceiptHandle,
			Attributes:        msg.Attributes,
			MessageAttributes: convertAttribute(msg.MessageAttributes),
		})
	}

	return messages
}

// convertAttribute turns an AWS type into `any` instead.
func convertAttribute(attributes map[string]types.MessageAttributeValue) map[string]any {
	attrs := make(map[string]any)

	for k, v := range attributes {
		if v.DataType == nil {
			continue
		}

		switch *v.DataType {
		case "String", "Number":
			attrs[k] = *v.StringValue
		case "Binary":
			attrs[k] = v.BinaryValue
		}
	}

	return attrs
}
