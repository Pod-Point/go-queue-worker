package formigo

import "time"

type Message interface {
	ReceivedAt() time.Time
	Content() any
	Id() any
}
