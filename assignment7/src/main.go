package main

import (
	"log"
	"net/http"
	"time"

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

/*
Synchronous payment bottleneck:
15 concurrent verifications
3 seconds each
=> theoretical throughput = 5 orders/sec
*/
var paymentSem = make(chan struct{}, 15)

/*
Local in-memory async queue for Phase 3 simulation.
Later, this will be replaced by SNS + SQS.
*/
var orderQueue = make(chan Order, 10000)

func verifyPayment(order Order) {
	log.Printf("Order %s waiting for payment slot", order.OrderID)

	paymentSem <- struct{}{}
	defer func() { <-paymentSem }()

	log.Printf("Order %s started payment verification", order.OrderID)
	time.Sleep(3 * time.Second)
	log.Printf("Order %s finished payment verification", order.OrderID)
}

func startBackgroundWorker() {
	go func() {
		for order := range orderQueue {
			log.Printf("Background worker picked up order %s", order.OrderID)

			order.Status = "processing"
			verifyPayment(order)
			order.Status = "completed"

			log.Printf("Background worker completed order %s", order.OrderID)
		}
	}()
}

func main() {
	startBackgroundWorker()

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Phase 1 synchronous endpoint
	router.POST("/orders/sync", func(c *gin.Context) {
		var order Order

		if err := c.ShouldBindJSON(&order); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		if order.CreatedAt.IsZero() {
			order.CreatedAt = time.Now()
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

	// Phase 3 asynchronous endpoint
	router.POST("/orders/async", func(c *gin.Context) {
		var order Order

		if err := c.ShouldBindJSON(&order); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		if order.CreatedAt.IsZero() {
			order.CreatedAt = time.Now()
		}

		order.Status = "pending"

		// enqueue immediately
		orderQueue <- order

		c.JSON(http.StatusAccepted, gin.H{
			"order_id": order.OrderID,
			"status":   "accepted",
			"mode":     "async",
		})
	})

	log.Println("Server running on port 8080")

	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}