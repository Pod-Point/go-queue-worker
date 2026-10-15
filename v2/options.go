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
// Default: 50
func WithWorkerConcurrency(concurrency int) OptionFunc {
	return func(manager *Manager) {
		manager.workerConcurrency = concurrency
	}
}

// WithFetchConcurrency will set the number of workers that will be continuously fetching messages from the given queue.
// This will block on messages being sent into workers if it is unreasonably high (e.g. equal to WorkerConcurrency).
//
// Default: 3
func WithFetchConcurrency(concurrency int) OptionFunc {
	return func(manager *Manager) {
		manager.fetchConcurrency = concurrency
	}
}

// WithFetchDelay will set the amount of time to sleep between fetching new messages.
// Default: 0ms
func WithFetchDelay(delay time.Duration) OptionFunc {
	return func(manager *Manager) {
		manager.fetchDelay = delay
	}
}

// WithReporter will set the error reporting function (e.g. logging, sentry).
// Default: nil
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

// WithLogger will set the internal logger for formigo.
// Default: nil
func WithLogger(logger *slog.Logger) OptionFunc {
	return func(manager *Manager) {
		manager.logger = logger
	}
}

// WithRetry will set a user-defined function to retry messages based on the message and the error.
// Note that the context is not included as it is always cancelled at this point.
// Also note that if you have a dead-letter queue configured, it will take precedence over this
// if the number of releases exceeds its pre-defined limit.
//
// Default: always retry on error and allow the message to hit a dead-letter queue.
func WithRetry(f func(Message, error) bool) OptionFunc {
	return func(manager *Manager) {
		manager.retryFunc = f
	}
}
