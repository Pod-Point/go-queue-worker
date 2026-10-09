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

// Client is a thin wrapper around an SQS client that can fetch, release and delete messages.
type Client interface {
	Fetch(context.Context) ([]Message, error)
	Delete(context.Context, Message) error
	Release(context.Context, Message) error
}

// Manager sets up a set of workers and processes an queue via them.
type Manager struct {
	client            Client
	pool              pond.Pool
	deadline          time.Duration
	fetchDelay        time.Duration
	fetchConcurrency  int
	workerConcurrency int
	queueSize         int
	consumer          func(context.Context, Message) error
	reporter          func(error)
	logger            *slog.Logger
}

func (m *Manager) submit(ctx context.Context, msg Message) error {
	m.Log(ctx, slog.LevelDebug, "processing message",
		slog.String("messageID", msg.ID),
		slog.String("receiptHandle", msg.ReceiptHandle),
	)

	err := m.consumer(ctx, msg)
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
}

func (m *Manager) withDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	var cancel func()

	if m.deadline > 0 {
		return context.WithDeadline(ctx, time.Now().Add(m.deadline))
	}

	return ctx, cancel
}

// Submit will push the message onto a consumer via the worker pool.
func (m *Manager) Submit(msg Message) error {
	m.pool.Submit(func() {
		ctx, cancel := m.withDeadline(context.Background())
		m.Log(ctx, slog.LevelDebug, "applying deadline",
			slog.String("messageID", msg.ID),
			slog.Duration("deadline", m.deadline),
		)

		// immediately cancel the context after returning, and replace it for deletion.
		err := m.submit(ctx, msg)
		cancel()

		// original error was a deadline expiry - do not delete and allow a second pass to resume.
		// the consumer really should have idempotency here.
		if errors.Is(err, context.DeadlineExceeded) {
			m.Log(ctx, slog.LevelDebug, "deadline exceeded - dropping message",
				slog.String("messageID", msg.ID),
				slog.Duration("deadline", m.deadline),
			)
			return
		}

		if err != nil {
			// in all other cases, we report this error.
			// the only "expected" error in this situation is
			m.Report(err)
		}

		// TODO: retry strategy (not present in v1, can be added with backwards compat in v2)
		// - retry.ExponentialBackoff
		// - retry.*
		// - formigo.WithRetry(func (ctx, msg) (bool, time.Duration))

		// finally: if we got this far, delete the message from the queue.
		// all other cases have us exit out and not try again.
		m.Delete(context.Background(), msg)
	})

	return nil
}

// Delete will call the Client's Delete method and remove a message from the queue.
// This may succeed with no error, but still not remove the message (e.g. it was not the most recent fetch of this message).
func (m *Manager) Delete(ctx context.Context, msg Message) {
	m.Log(ctx, slog.LevelDebug, "deleting message",
		slog.String("messageID", msg.ID),
		slog.String("receiptHandle", msg.ReceiptHandle),
	)

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
			m.Log(ctx, slog.LevelDebug, "exiting fetch instance")
			return nil
		default:
			m.Log(ctx, slog.LevelDebug, "fetching messages")
			messages, err := m.client.Fetch(ctx)
			if err != nil {
				m.Report(err) // report errors with the initial fetch.
				continue
			}

			m.Log(ctx, slog.LevelDebug, "fetched messages", slog.Int("count", len(messages)))

			// in the event that all workers are busy, this will block until one becomes available.
			// these will also give an automatic error of pond.ErrPoolStopped if the workers have stopped.
			for _, msg := range messages {
				if err := m.Submit(msg); err != nil {
					// in this instance we should release
					// the tasks in-flight via SQS immediately.
					if errors.Is(err, pond.ErrPoolStopped) {
						return m.release(context.Background(), msg)
					}

					m.Report(err)
				}
			}

			m.Log(ctx, slog.LevelDebug, "waiting before polling", slog.Duration("delay", m.fetchDelay))

			// allow configuring a delay after fetching messages
			// this may help prevent excessive pressure on AWS.
			if m.fetchDelay > 0 {
				time.Sleep(m.fetchDelay)
			}
		}
	}
}

func (m *Manager) release(ctx context.Context, msg Message) error {
	m.Log(ctx, slog.LevelWarn, "releasing message back to sqs", slog.String("messageID", msg.ID))

	return m.client.Release(ctx, msg)
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

	m.pool = pond.NewPool(m.workerConcurrency,
		pond.WithContext(ctx),
		pond.WithQueueSize(m.queueSize),
		pond.WithNonBlocking(false),
	)

	// set up the fetchers to feed the workers and start them
	fetcher := pond.NewPool(m.fetchConcurrency, pond.WithContext(ctx))
	for i := range m.fetchConcurrency {
		m.Log(ctx, slog.LevelInfo, "starting fetcher", slog.Int("id", i))

		fetcher.Submit(func() {
			if err := m.Fetch(ctx); err != nil {
				m.Report(err)
			}
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

// Pool exists to expose the internal workings of the queue worker for testing and metrics.
// Manipulating existing state of the queue while it is running is explicitly undefined.
func (m *Manager) Pool() pond.Pool {
	return m.pool
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
		fetchDelay:        0,
		fetchConcurrency:  2,
		workerConcurrency: 20,
		deadline:          time.Second * 30,
		queueSize:         pond.Unbounded,
	}
}
