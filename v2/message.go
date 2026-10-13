package formigo

import (
	"encoding/json"
	"log/slog"
)

const (
	messageID     = "messageID"
	receiptHandle = "receiptHandle"
)

type MessageLogger interface {
	LogArguments() []slog.Attr
}

type Message struct {
	ID                string
	Attributes        map[string]string
	Body              string
	MessageAttributes map[string]any
	ReceiptHandle     string
}

// Decode attempts to decode a message's Body using json.Unmarshal.
func (m Message) Decode(value any) error {
	return json.Unmarshal([]byte(m.Body), &value)
}

// LogArguments grabs the common logging arguments returned for a message.
func (m Message) LogArguments() []slog.Attr {
	return []slog.Attr{
		slog.String(messageID, m.ID),
		slog.String(receiptHandle, m.ReceiptHandle),
	}
}
