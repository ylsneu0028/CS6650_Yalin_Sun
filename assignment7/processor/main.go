package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
	ctx := context.Background()

	queueURL := os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("SQS_QUEUE_URL is required")
	}

	workerCount := 1
	if w := os.Getenv("WORKER_COUNT"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			workerCount = n
		}
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	client := sqs.NewFromConfig(cfg)

	// Semaphore to limit concurrent message handlers
	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	log.Printf("processor started, queue=%s workers=%d", queueURL, workerCount)

	for {
		out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20,
		})
		if err != nil {
			log.Printf("ReceiveMessage error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, msg := range out.Messages {
			if msg.MessageId == nil || msg.ReceiptHandle == nil {
				continue
			}
			msgID := *msg.MessageId
			receiptHandle := *msg.ReceiptHandle

			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()

				log.Printf("[PROCESS] order message %s started", msgID)
				time.Sleep(3 * time.Second) // simulate payment verification

				_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(queueURL),
					ReceiptHandle: aws.String(receiptHandle),
				})
				if err != nil {
					log.Printf("[PROCESS] DeleteMessage %s error: %v", msgID, err)
					return
				}
				log.Printf("[PROCESS] order message %s completed", msgID)
			}()
		}
	}
}
