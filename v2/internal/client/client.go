package client

import (
	"context"

	"github.com/Pod-Point/go-queue-worker/v2/internal/messages"
)

type MessageReceiver interface {
	ReceiveMessages(ctx context.Context) ([]messages.Message, error)
}

type MessageDeleter interface {
	DeleteMessages(messages []messages.Message) error
}

type Client interface {
	MessageReceiver
	MessageDeleter
}
