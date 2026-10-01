// Package formigo exposes an extremely simple way to run SQS messages inside a worker pool with capped concurrency.
// It also handles graceful shutdown, and ensures messages in-flight are completed on context cancellation.
package formigo

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/alitto/pond/v2"
)

var (
	ErrConsumerMissing = errors.New("formigo/v2: consumer is not set. Use .WithConsumer()")
)

// Fetcher condenses RetrieveMessage into the smallest possible interface.
type Fetcher interface {
	Fetch(context.Context) ([]Message, error)
}

// Deleter condenses DeleteMessage into the smallest possible interface.
type Deleter interface {
	Delete(context.Context, Message) error
}

// Client is a thin wrapper around an SQS client that can fetch and delete messages.
type Client interface {
	Fetcher
	Deleter
}

// Manager sets up a set of workers and processes an queue via them.
type Manager struct {
	client            Client
	pool              pond.Pool
	deadline          time.Duration
	fetchDelay        time.Duration
	fetchConcurrency  int
	workerConcurrency int
	consumer          func(context.Context, Message) error
	reporter          func(error)
	logger            *slog.Logger
}

// Submit will push the message onto a consumer via the worker pool.
func (m *Manager) Submit(msg Message) (err error) {
	m.pool.SubmitErr(func() error {
		ctx := context.Background()

		// make sure this consumer deletes the message from the queue when deferring.
		// this one uses the background context with no deadline, and reports its own errors.
		defer m.Delete(ctx, msg, err)

		m.Log(ctx, slog.LevelDebug, "processing message",
			slog.String("messageID", msg.ID),
			slog.String("receiptHandle", msg.ReceiptHandle),
		)

		if m.deadline > 0 {
			m.Log(ctx, slog.LevelDebug, "applying deadline",
				slog.String("messageID", msg.ID),
				slog.Duration("deadline", m.deadline),
			)

			var cancel func()
			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(m.deadline))
			defer cancel()
		}

		err = m.consumer(ctx, msg)

		if err != nil {
			m.Log(ctx, slog.LevelInfo, "message: errored",
				slog.String("messageID", msg.ID),
				slog.String("error", err.Error()),
			)
		} else {
			m.Log(ctx, slog.LevelInfo, "message: processed",
				slog.String("messageID", msg.ID),
			)
		}

		return err
	})

	return nil
}

// Delete will call the Client's Delete method and remove a message from the queue.
// This may succeed with no error, but still not remove the message (e.g. it was not the most recent fetch of this message).
func (m *Manager) Delete(ctx context.Context, msg Message, err error) {
	m.Log(ctx, slog.LevelDebug, "deleting message",
		slog.String("messageID", msg.ID),
		slog.String("receiptHandle", msg.ReceiptHandle),
	)

	if err != nil {
		m.Report(err)
	}

	// if err is a failure, we can choose if we retry here.
	// TODO: retry strategy

	if err := m.client.Delete(ctx, msg); err != nil {
		m.Log(ctx, slog.LevelError, "failed to delete message",
			slog.String("messageID", msg.ID),
			slog.String("error", err.Error()),
		)
		m.Report(err)
	} else {
		m.Log(ctx, slog.LevelInfo, "message deleted",
			slog.String("messageID", msg.ID),
		)
	}
}

// Fetch will run a continuous loop of querying SQS for messages.
func (m *Manager) Fetch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			m.Log(ctx, slog.LevelDebug, "fetching messages")
			messages, err := m.client.Fetch(ctx)
			if err != nil {
				m.Report(err) // report errors with the initial fetch.
				continue
			}

			m.Log(ctx, slog.LevelDebug, "fetched messages", slog.Int("count", len(messages)))

			for _, msg := range messages {
				if err := m.Submit(msg); err != nil {
					m.Report(err)
				}
			}

			m.Log(ctx, slog.LevelDebug, "waiting before polling", slog.Duration("delay", m.fetchDelay))

			// allow configuring a delay after fetching messages
			// this may help prevent excessive pressure on AWS.
			time.Sleep(m.fetchDelay)
		}
	}
}

// Report will report an error whenever a func for it is set.
func (m *Manager) Report(err error) {
	if m.reporter != nil {
		m.reporter(err)
	}
}

// Run will start up a worker pool and start processing a queue.
// It must exit via the context, e.g. via signal.NotifyContext or cancel().
// Using context.Background will never safely exit.
//
// This will error if a consumer is not set.
func (m *Manager) Run(ctx context.Context) error {
	if m.consumer == nil {
		return ErrConsumerMissing
	}

	m.pool = pond.NewPool(m.workerConcurrency, pond.WithContext(ctx))

	// set up the fetchers to feed the workers and start them
	fetcher := pond.NewPool(m.fetchConcurrency, pond.WithContext(ctx))
	for i := range m.fetchConcurrency {
		m.Log(ctx, slog.LevelInfo, "starting fetcher", slog.Int("id", i))

		fetcher.SubmitErr(func() error {
			return m.Fetch(ctx)
		})
	}

	select {
	case <-ctx.Done():
		m.Log(ctx, slog.LevelInfo, "stopping fetchers")

		// stop the fetcher first to be sure it has drained
		fetcher.StopAndWait()

		m.Log(ctx, slog.LevelInfo, "stopping workers")

		// stop and drain the worker pool
		m.pool.StopAndWait()
		return ctx.Err()
	}
}

// Log will send a log record to a configured slog.Logger, if set.
func (m *Manager) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if m.logger != nil {
		m.logger.Log(ctx, level, msg, args...)
	}
}

// NewManager will create a new *Manager with the given client and options.
func NewManager(client Client, opts ...Option) *Manager {
	manager := NewDefaultManager(client)

	for _, opt := range opts {
		opt.Apply(manager)
	}

	manager.Log(context.Background(), slog.LevelInfo, "applied options",
		slog.Int("workerConcurrency", manager.workerConcurrency),
		slog.Int("fetchConcurrency", manager.fetchConcurrency),
		slog.Duration("fetchDelay", manager.fetchDelay),
		slog.Duration("deadline", manager.deadline),
	)

	return manager
}

// NewDefaultManager returns a Manager with all defaults set in the options.
func NewDefaultManager(client Client) *Manager {
	return &Manager{
		client:            client,
		fetchDelay:        time.Millisecond * 500,
		fetchConcurrency:  2,
		workerConcurrency: 20,
		deadline:          time.Second * 30,
	}
}
