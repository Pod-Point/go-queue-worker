// Package main is a test producer for a localstack queue.
// This is not meant to be a production config, though it is set up exactly as you would use it there.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("unable to load config: %v\n", err)
		os.Exit(2)
	}

	if len(os.Args[1:]) != 2 {
		fmt.Println("USAGE: ./echo [level] \"[message]\"")
	}

	level := os.Args[1]
	body := os.Args[2]

	b, err := json.Marshal(struct {
		Level string `json:"level"`
		Body  string `json:"message"`
	}{
		Level: level,
		Body:  body,
	})
	if err != nil {
		log.Fatal(err)
	}

	client := sqs.NewFromConfig(cfg)

	// duplicate for testing
	for range 10 {
		output, err := client.SendMessage(ctx, &sqs.SendMessageInput{
			MessageBody: aws.String(string(b)),
			QueueUrl:    aws.String("http://sqs.eu-west-1.localhost.localstack.cloud:4566/000000000000/formigo-echo-testing"),
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("message id: %s\n", *output.MessageId)
	}
}
