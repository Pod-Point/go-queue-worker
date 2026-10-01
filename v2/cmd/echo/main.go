// Package main is a test consumer for a localstack queue.
// This is not meant to be a production config, though it is set up exactly as you would use it there.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	formigo "github.com/Pod-Point/go-queue-worker/v2"
)

type Message struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGILL, syscall.SIGHUP)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("unable to load config: %v\n", err)
		os.Exit(2)
	}

	client := formigo.NewSQSClient(sqs.NewFromConfig(cfg), &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String("http://sqs.eu-west-1.localhost.localstack.cloud:4566/000000000000/formigo-echo-testing"),
		MaxNumberOfMessages: 5,
		VisibilityTimeout:   6,
		WaitTimeSeconds:     10,
	})

	manager := formigo.NewManager(client,
		formigo.WithDeadline(time.Second*4),
		formigo.WithFetchConcurrency(2),
		formigo.WithFetchDelay(time.Second*1),
		formigo.WithWorkerConcurrency(3),
		formigo.WithReporter(func(err error) {
			slog.Error(err.Error(), slog.String("type", fmt.Sprintf("%T", err)))
		}),
		formigo.WithConsumer(func(ctx context.Context, msg formigo.Message) error {
			var body Message
			if err := msg.Decode(&body); err != nil {
				return err
			}

			switch body.Level {
			case "error":
				return errors.New(body.Message)
			case "info":
				slog.InfoContext(ctx, body.Message)
			case "warn", "warning":
				slog.WarnContext(ctx, body.Message)
			case "debug":
				slog.DebugContext(ctx, body.Message)
			}

			return nil
		}),
		formigo.WithLogger(slog.New(
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}),
		)),
	)

	if err := manager.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
