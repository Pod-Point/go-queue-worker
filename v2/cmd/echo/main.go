// Package main is a test consumer for a localstack queue.
// This is not meant to be a production config, though it is set up exactly as you would use it there.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
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
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("unable to load config: %v\n", err)
		os.Exit(2)
	}

	client := formigo.NewSQSClient(sqs.NewFromConfig(cfg), &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String("http://sqs.eu-west-1.localhost.localstack.cloud:4566/000000000000/formigo-v2-testing"),
		MaxNumberOfMessages: 10,
		VisibilityTimeout:   6,
		WaitTimeSeconds:     10,
	})

	var count atomic.Int64

	manager := formigo.NewManager(client,
		formigo.WithDeadline(time.Second*4),
		formigo.WithFetchConcurrency(2),
		formigo.WithWorkerConcurrency(100),
		formigo.WithQueueSize(0),
		formigo.WithReporter(func(err error) {
			slog.Error(err.Error(), slog.String("type", fmt.Sprintf("%T", err)))
		}),
		formigo.WithConsumer(func(ctx context.Context, msg formigo.Message) error {
			c := count.Add(1)

			time.Sleep(time.Millisecond * 50 * time.Duration(c%15))

			fmt.Println(msg.Body)

			return nil
		}),
		formigo.WithLogger(slog.New(
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelWarn,
			}),
		)),
	)

	if err := manager.Run(ctx); err != nil {
		//
	}

	pool := manager.Pool()

	metrics := &Metrics{
		Count:      count.Load(),
		Completed:  pool.CompletedTasks(),
		Cancelled:  pool.CanceledTasks(),
		Submitted:  pool.SubmittedTasks(),
		Failed:     pool.FailedTasks(),
		Dropped:    pool.DroppedTasks(),
		Successful: pool.SuccessfulTasks(),
		Waiting:    pool.WaitingTasks(),
	}

	b, err := json.Marshal(metrics)
	if err != nil {
		fmt.Printf("err: %v\n\n%v\n", err, metrics)
		os.Exit(1)
	}

	fmt.Println(string(b))
}

type Metrics struct {
	Count      int64  `json:"count"`
	Completed  uint64 `json:"completed"`
	Cancelled  uint64 `json:"cancelled"`
	Submitted  uint64 `json:"submitted"`
	Failed     uint64 `json:"failed"`
	Dropped    uint64 `json:"dropped"`
	Successful uint64 `json:"successful"`
	Waiting    uint64 `json:"waiting"`
}
