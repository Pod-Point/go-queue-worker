package client

import "github.com/Pod-Point/go-queue-worker/internal/messages"

type MessageReceiver interface {
	ReceiveMessages() ([]messages.Message, error)
}

type MessageDeleter interface {
	DeleteMessages(messages []messages.Message) error
}

type Client interface {
	MessageReceiver
	MessageDeleter
}
