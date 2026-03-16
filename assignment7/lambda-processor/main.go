package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Order matches the receiver's Order struct (subset used for logging).
type Order struct {
	OrderID    string `json:"order_id"`
	CustomerID int    `json:"customer_id"`
	Status     string `json:"status"`
}

func handleSNS(ctx context.Context, evt events.SNSEvent) error {
	for _, rec := range evt.Records {
		msg := rec.SNS.Message
		var order Order
		if err := json.Unmarshal([]byte(msg), &order); err != nil {
			log.Printf("[LAMBDA] invalid message (not order JSON): %s", msg)
			continue
		}
		log.Printf("[LAMBDA] order %s processing started", order.OrderID)
		time.Sleep(3 * time.Second) // same 3s payment simulation as ECS processor
		log.Printf("[LAMBDA] order %s processing completed", order.OrderID)
	}
	return nil
}

func main() {
	lambda.Start(handleSNS)
}
