package formigo

import (
	"context"
	"time"

	"github.com/alitto/pond/v2"
)

// Fetcher condenses RetrieveMessage into the smallest possible interface.
type Fetcher interface {
	Fetch(context.Context) ([]Message, error)
}

// Deleter condenses DeleteMessage into the smallest possible interface.
type Deleter interface {
	Delete(context.Context, Message) error
}

type Client interface {
	Fetcher
	Deleter
}

// Manager sets up a set of workers and processes an queue via them.
type Manager struct {
	client     Client
	pool       pond.Pool
	deadline   time.Duration
	fetchDelay time.Duration

	deleter  func(context.Context, Message) error
	consumer func(context.Context, Message) error
	reporter func(error)
}

// Submit will push the message onto a consumer via the worker pool.
func (m *Manager) Submit(msg Message) error {
	m.pool.SubmitErr(func() error {
		ctx := context.Background()

		// make sure this consumer deletes the message from the queue when deferring.
		// this one uses the background context with no deadline, and reports its own errors.
		defer m.Delete(ctx, msg)

		if m.deadline > 0 {
			var cancel func()
			ctx, cancel = context.WithDeadline(ctx, time.Now().Add(m.deadline))
			defer cancel()
		}

		if err := m.consumer(ctx, msg); err != nil {
			m.reporter(err)
			return err
		}

		return nil
	})

	return nil
}

// Delete will call the Client's Delete method, skipping any batch deletes.
func (m *Manager) Delete(ctx context.Context, msg Message) {
	if err := m.client.Delete(ctx, msg); err != nil {
		m.reporter(err)
	}
}

func (m *Manager) Fetch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
		default:
			messages, err := m.client.Fetch(ctx)
			if err != nil {
				m.Report(err) // report errors with the initial fetch.
				continue
			}

			for _, msg := range messages {
				if err := m.Submit(msg); err != nil {
					m.Report(err)
				}
			}

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
// It must exit via the context, e.g. via signal.NotifyContext.
// Using context.Background will never safely exit.
func (m *Manager) Run(ctx context.Context) error {
	m.pool = pond.NewPool(20, pond.WithContext(ctx))

	// set up the fetchers to feed the workers and start them
	fetcher := pond.NewPool(2, pond.WithContext(ctx))
	for range 2 {
		fetcher.SubmitErr(func() error {
			return m.Fetch(ctx)
		})
	}

	select {
	case <-ctx.Done():
		// stop the fetcher first to be sure it has drained
		fetcher.StopAndWait()

		// stop and drain the worker pool
		m.pool.StopAndWait()
		return ctx.Err()
	}
}
