# Formigo - distributed SQS worker pools.

> This library relies heavily on [`github.com/alitto/pond/v2`](https://github.com/alitto/pond), a very flexible and well-tested worker pool library.

Formigo is a fast, reliable worker pool consumer for SQS.

Basic features of Pond:

- automatic scaling to available resources/limits based on incoming queue pressure.
- fire & forget queue submission
- fire & **wait for a response** submission to a queue (e.g. for dependent jobs, or deferred behaviour).
- clean, graceful exit when shutting down.

## Usage

See `cmd/echo/main.go` for a complete example of how to use v2.

### Consumers

Your consumer should adhere to the following:

```go
import formigo "github.com/Pod-Point/go-queue-worker/v2"

func (context.Context, formigo.Message) error
```

A `Decode` method is provided on formigo.Message to help ensure proper decoding with JSON SQS messages:

```go
type Message struct {
	Foo string `json:"foo"`
}

func (ctx context.Context, msg formigo.Message) error {
	var body Message
	if err := msg.Decode(&body); err != nil {
		return err
	}
	// pass to your internal consumer, etc.
}
```

### Error Reporting

If you need errors to be sent to Sentry, structured logging, etc, feel free to use `WithReporter` when setting up a manager.

```go
formigo.WithReporter(func(err error) {
	sentry.CaptureException(err)
})
```

### Other Options

All other options should be fairly self-explanatory, and have godocs and defaults.

## Shutting Down

Ensure the context passed to `manager.Run(ctx)` is cancelable, preferrably via `signal.NotifyContext`.

If it isn't, the program will have no exit condition until fully terminated by the operating system.

See the example consumer to see how this is done.