package formigo

import (
	"context"
	"log/slog"
	"time"
)

type Option interface {
	Apply(*Manager)
}

type OptionFunc func(*Manager)

func (f OptionFunc) Apply(manager *Manager) {
	f(manager)
}

// WithDeadline sets the context deadline used for consumer processes when they are given a message.
// Ensure this is lower than VisibilityTimeout in SQS, or you may process the same message twice.
//
// Default: 30s
func WithDeadline(deadline time.Duration) OptionFunc {
	return func(manager *Manager) {
		manager.deadline = deadline
	}
}

// WithWorkerConcurrency will set the number of workers used to run a consumer process.
// Set this high enough so that you can process a reasonably high number of messages,
// but not so high that you throttle/cpu-limit the other workers.
//
// Default: 20
func WithWorkerConcurrency(concurrency int) OptionFunc {
	return func(manager *Manager) {
		manager.workerConcurrency = concurrency
	}
}

// WithQueueSize will set the maximum number of requests in flight before adds to the queue are rejected.
// Note that fetches will be stopped while the queue is full, but any that exceed the queue size will be
// dropped without being deleted, and have to wait until the VisibilityTimeout expires.
//
// It should be kept at a value that is manageable per-node, but should either stay constant or decrease over time.
// If the WaitingTasks() output from Pool() shows an increasing trend, the queue is in an unsustainable state.
//
// Default: pond.Unbounded (math.MaxInt)
func WithQueueSize(size int) OptionFunc {
	return func(manager *Manager) {
		manager.queueSize = size
	}
}

// WithFetchConcurrency will set the number of workers that will be continuously fetching messages from the given queue.
// Do not set this significantly higher than needed (3-4 is sufficient) or you may grow the queue backlog.
//
// Default: 2
func WithFetchConcurrency(concurrency int) OptionFunc {
	return func(manager *Manager) {
		manager.fetchConcurrency = concurrency
	}
}

// WithFetchDelay will set the amount of time to sleep between fetching new messages.
// Default: 500ms
func WithFetchDelay(delay time.Duration) OptionFunc {
	return func(manager *Manager) {
		manager.fetchDelay = delay
	}
}

// WithReporter will set the error reporting function (e.g. log
func WithReporter(f func(error)) OptionFunc {
	return func(manager *Manager) {
		manager.reporter = f
	}
}

// WithConsumer sets the consumer to use when processing messages.
// Required; panics if not set since this is a dev issue.
func WithConsumer(f func(context.Context, Message) error) OptionFunc {
	return func(manager *Manager) {
		manager.consumer = f
	}
}

func WithLogger(logger *slog.Logger) OptionFunc {
	return func(manager *Manager) {
		manager.logger = logger
	}
}
