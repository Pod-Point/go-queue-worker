package formigo

import (
	"log"
	"time"

	"github.com/Pod-Point/go-queue-worker/v2/internal/client"
)

const (
	defaultConcurrency          = 100
	defaultRetrievers           = 1
	defaultDeleterBufferSize    = 10
	defaultDeleterBufferTimeout = time.Millisecond * 500
)

type DeleterConfiguration struct {
	BufferSize    int
	BufferTimeout time.Duration
}

// The BatchConsumerBufferConfiguration defines a buffer which is consumed by the worker when either
// the buffer is full or the timeout has passed since the first message got added.
type BatchConsumerBufferConfiguration struct {
	// Max number of messages that the buffer can contain.
	// Default: 10.
	Size int

	// Time after which the buffer gets processed, no matter whether it is full or not.
	// This value MUST be smaller tha VisibilityTimeout in the
	// RetrieveMessageConfiguration + the maximum processing time of the handler.
	// If this is not set correctly, the same message could be processed multiple times.
	// Default: 1s.
	Timeout time.Duration
}

type MessageConsumerConfiguration struct {
	Handler MessageHandler
}

type BatchConsumerConfiguration struct {
	Handler      BatchHandler
	BufferConfig BatchConsumerBufferConfiguration
}

type Configuration struct {
	// Client is a queue client.
	Client client.Client

	// Concurrency is the number of Go routines that process the messages from the Queue.
	// The higher this value, the more Go routines are spawned to process the messages.
	// Using a high value can be useful when the Handler of the consumer perform slow I/O operations.
	// Default: 100.
	Concurrency int

	// Retrievers is the number of Go routines that retrieve messages from the Queue.
	// The higher this value, the more Go routines are spawned to read the messages from the
	// queue and provide them to the worker's consumers.
	// Using a high value can be useful when the network is slow or when consumers are quicker
	// than retrievers.
	// Default: 1.
	Retrievers int

	// ReportFunc will log/report an error as needed by the user.
	// Default: logs errors. Set to an empty func to disable.
	ReportFunc func(error)

	// The messages Consumer.
	Consumer Consumer

	// Configuration for the deleter
	DeleterConfig DeleterConfiguration
}

func setWorkerConfigValues(config Configuration) Configuration {
	if config.Retrievers == 0 {
		config.Retrievers = defaultRetrievers
	}

	if config.Concurrency == 0 {
		config.Concurrency = defaultConcurrency
	}

	if config.ReportFunc == nil {
		config.ReportFunc = func(err error) {
			log.Println("ERROR", err)
		}
	}

	if config.DeleterConfig.BufferSize == 0 {
		config.DeleterConfig.BufferSize = defaultDeleterBufferSize
	}

	if config.DeleterConfig.BufferTimeout == 0 {
		config.DeleterConfig.BufferTimeout = defaultDeleterBufferTimeout
	}

	return config
}
