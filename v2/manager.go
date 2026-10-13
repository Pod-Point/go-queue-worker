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
	consumer          func(context.Context, Message) error
	reporter          func(error)
	logger            *slog.Logger
}

// NewManager will create a new *Manager with the given client and options.
func NewManager(client Client, opts ...Option) *Manager {
	manager := NewDefaultManager(client)

	for _, opt := range opts {
		opt.Apply(manager)
	}

	manager.debug(context.Background(), "applied options",
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
		fetchConcurrency:  3,
		workerConcurrency: 50,
		deadline:          time.Second * 30,
	}
}

// Submit will push the message onto a consumer via the worker pool.
func (m *Manager) Submit(msg Message) error {
	m.pool.Submit(func() {
		ctx, cancel := m.withDeadline(context.Background())
		m.debug(ctx, "applying deadline", msg, slog.Duration("deadline", m.deadline))

		// immediately cancel the context after returning, and replace it for deletion.
		err := m.submit(ctx, msg)
		cancel()

		m.handleError(msg, err)
	})

	return nil
}

// Fetch will run a continuous loop of querying SQS for messages.
func (m *Manager) Fetch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			m.debug(ctx, "exiting fetch instance")
			return nil
		default:
			m.debug(ctx, "fetching messages")
			messages, err := m.client.Fetch(ctx)
			if err != nil {
				m.Report(err) // report errors with the initial fetch.
				continue
			}

			m.debug(ctx, "fetched messages", slog.Int("count", len(messages)))

			// in the event that all workers are busy, this will block until one becomes available.
			// these will also give an automatic error of pond.ErrPoolStopped if the workers have stopped.
			for _, msg := range messages {
				if err := m.Submit(msg); err != nil {
					// in this instance we should release
					// the tasks in-flight via SQS immediately.
					if errors.Is(err, pond.ErrPoolStopped) {
						return m.Release(context.Background(), msg)
					}

					m.Report(err)
				}
			}

			// allow configuring a delay after fetching messages
			// this may help prevent excessive pressure on AWS.
			if m.fetchDelay > 0 {
				time.Sleep(m.fetchDelay)
			}
		}
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

	// we do not make queue size/blocking configurable here because it's a core feature.
	// if we allow a queue size, then we can lose messages from the fetching workers.
	m.pool = pond.NewPool(
		m.workerConcurrency,
		pond.WithContext(ctx),
		pond.WithQueueSize(0),
		pond.WithNonBlocking(false),
	)

	// set up the fetchers to feed the workers and start them
	fetcher := pond.NewPool(m.fetchConcurrency, pond.WithContext(ctx))
	for i := range m.fetchConcurrency {
		m.info(ctx, "starting sqs message polling", slog.Int("id", i))

		fetcher.Submit(func() {
			if err := m.Fetch(ctx); err != nil {
				m.Report(err)
			}
		})
	}

	select {
	case <-ctx.Done():
		m.info(ctx, "stopping sqs message polling")

		// stop the fetcher first to be sure it has drained
		fetcher.StopAndWait()

		m.info(ctx, "stopping queue workers")

		// stop and drain the worker pool
		m.pool.StopAndWait()
		return ctx.Err()
	}
}

// Pool exists to expose the internal workings of the queue worker for testing and metrics.
// Manipulating existing state of the queue while it is running is explicitly undefined.
func (m *Manager) Pool() pond.Pool {
	return m.pool
}

func (m *Manager) Release(ctx context.Context, msg Message) error {
	m.info(ctx, "releasing message back to sqs", msg)

	return m.client.Release(ctx, msg)
}

// Delete will call the Client's Delete method and remove a message from the queue.
// This may succeed with no error, but still not remove the message (e.g. it was not the most recent fetch of this message).
func (m *Manager) Delete(ctx context.Context, msg Message) {
	m.debug(ctx, "deleting message", msg)

	if err := m.client.Delete(ctx, msg); err != nil {
		m.error(ctx, "failed to delete message", msg, err)
		m.Report(err)
	} else {
		m.debug(ctx, "message deleted", msg)
	}
}

// Report will report an error whenever a func for it is set.
func (m *Manager) Report(err error) {
	if m.reporter != nil {
		m.reporter(err)
	}
}

func (m *Manager) shouldRetry(msg Message, err error) bool {
	// TODO: add a user-defined function here to hook into this process.
	// TODO: retry strategy should use this function.

	// func(msg Message, err error) bool

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true
	default:
		return false
	}
}

func (m *Manager) handleError(msg Message, err error) {
	ctx := context.Background()

	// normal operation - exit early.
	if err == nil {
		m.Delete(ctx, msg)
		return
	}

	// check if we should retry this message
	if m.shouldRetry(msg, err) {
		m.debug(ctx, "retrying message", msg, err)

		// release this message back to the queue immediately.
		if err := m.Release(ctx, msg); err != nil {
			m.Report(err)
		}

		return
	}

	// if we are not retrying, then this error is final.
	m.Delete(context.Background(), msg)
}

func (m *Manager) submit(ctx context.Context, msg Message) error {
	m.debug(ctx, "processing message", msg)

	err := m.consumer(ctx, msg)
	if err != nil {
		m.error(ctx, "message: errored", msg, err)
	} else {
		m.info(ctx, "message: processed", msg)
	}

	return err
}

func (m *Manager) withDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if m.deadline > 0 {
		return context.WithDeadline(ctx, time.Now().Add(m.deadline))
	}

	return context.WithCancel(ctx)
}
