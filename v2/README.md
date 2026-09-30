# Formigo - distributed SQS worker pools.

> This library relies heavily on [`github.com/alitto/pond/v2`](https://github.com/alitto/pond), a very flexible and well-tested worker pool library.

Formigo is a fast, reliable worker pool consumer for SQS.

Basic features of Pond:

- automatic scaling to available resources/limits based on incoming queue pressure.
- fire & forget queue submission
- fire & **wait for a response** submission to a queue (e.g. for dependent jobs, or deferred behaviour).
- clean, graceful exit when shutting down.

## Usage

In `main()`, you'll generally have the following:

```go
package main

import (
	"context"
	"log"
	"time"

	"libs/go/application"

	formigo "github.com/Pod-Point/go-queue-worker/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
	// app here has a context that uses signal.NotifyContext to catch SIGTERM/SIGKILL/SIGHUP etc.
	app := application.New().HandlesSigTerm()

	app.Run(func(ctx context.Context) error {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return err
		}

		client := formigo.NewSQSClient(sqs.NewFromConfig(cfg), &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String("https://sqs.eu-west-1.amazonaws.com/123456789012/my-queue"),
			MaxNumberOfMessages: 10,
			VisibilityTimeout:   30,
			WaitTimeSeconds:     20,
		})
		
		manager := formigo.NewManager(client,
			formigo.WithDeadline(time.Second * 45),
			formigo.WithFetchConcurrency(2),
			formigo.WithFetchDelay(time.Second * 1),
			formigo.WithWorkerConcurrency(20),
			formigo.WithReporter(func(err error) {
				log.Print(err) // send to sentry, structured logging, etc
            }),
			formigo.WithConsumer(func(ctx context.Context, msg formigo.Message) error {
				return nil
            }),
        )
		
		return manager.Run(ctx)
	})
}

```