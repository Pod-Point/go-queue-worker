package formigo

import (
	"encoding/json"
)

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
