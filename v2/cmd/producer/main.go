// Package main is a test producer for a localstack queue.
// This is not meant to be a production config.
package main

import (
	"context"
	"log"
	"os"
	"strconv"

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

	client := sqs.NewFromConfig(cfg)

	// duplicate for testing
	for i := range 10000 {
		_, err := client.SendMessage(ctx, &sqs.SendMessageInput{
			MessageBody: aws.String(strconv.Itoa(i)),
			QueueUrl:    aws.String("http://sqs.eu-west-1.localhost.localstack.cloud:4566/000000000000/formigo-v2-testing"),
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}
