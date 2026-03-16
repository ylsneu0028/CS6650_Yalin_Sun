package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gin-gonic/gin"
)

type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"` // pending, processing, completed
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// sync payment bottleneck: 15 concurrent verifications, each takes 3s
var paymentSem = make(chan struct{}, 15)

func verifyPayment(order Order) {
	log.Printf("[SYNC] order %s waiting for payment slot", order.OrderID)

	paymentSem <- struct{}{}
	defer func() { <-paymentSem }()

	log.Printf("[SYNC] order %s started payment verification", order.OrderID)
	time.Sleep(3 * time.Second)
	log.Printf("[SYNC] order %s finished payment verification", order.OrderID)
}

func buildSNSClient(ctx context.Context) (*sns.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return sns.NewFromConfig(cfg), nil
}

func main() {
	ctx := context.Background()

	topicARN := os.Getenv("SNS_TOPIC_ARN")
	if topicARN == "" {
		log.Fatal("SNS_TOPIC_ARN is required")
	}

	snsClient, err := buildSNSClient(ctx)
	if err != nil {
		log.Fatalf("failed to create SNS client: %v", err)
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"mode":   "receiver",
		})
	})

	// Phase 1 sync endpoint
	router.POST("/orders/sync", func(c *gin.Context) {
		var order Order

		if err := c.ShouldBindJSON(&order); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		if order.CreatedAt.IsZero() {
			order.CreatedAt = time.Now().UTC()
		}

		order.Status = "processing"
		verifyPayment(order)
		order.Status = "completed"

		c.JSON(http.StatusOK, gin.H{
			"order_id": order.OrderID,
			"status":   order.Status,
			"mode":     "sync",
		})
	})

	// Phase 3 async endpoint: publish to SNS, return 202 immediately
	router.POST("/orders/async", func(c *gin.Context) {
		var order Order

		if err := c.ShouldBindJSON(&order); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		if order.CreatedAt.IsZero() {
			order.CreatedAt = time.Now().UTC()
		}

		order.Status = "pending"

		payload, err := json.Marshal(order)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to serialize order",
			})
			return
		}

		_, err = snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(topicARN),
			Message:  aws.String(string(payload)),
		})
		if err != nil {
			log.Printf("failed to publish order %s to SNS: %v", order.OrderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to publish order event",
			})
			return
		}

		log.Printf("[ASYNC] published order %s to SNS", order.OrderID)

		c.JSON(http.StatusAccepted, gin.H{
			"order_id": order.OrderID,
			"status":   "accepted",
			"mode":     "async",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("receiver server running on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}