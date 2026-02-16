package main

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// ===== Models (match api.yaml exactly) =====

type Product struct {
	ProductID    int32  `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int32  `json:"category_id"`
	Weight       int32  `json:"weight"`
	SomeOtherID  int32  `json:"some_other_id"`
}

type ErrorResp struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ===== In-memory store (map + RWMutex) =====

type Store struct {
	mu       sync.RWMutex
	products map[int32]Product
}

func NewStore() *Store {
	return &Store{products: make(map[int32]Product)}
}

func (s *Store) Get(id int32) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

func (s *Store) Upsert(id int32, p Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[id] = p
}

// ===== Helpers =====

func parseProductIDParam(c *gin.Context) (int32, bool) {
	raw := c.Param("productId")
	id64, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResp{
			Error:   "INVALID_INPUT",
			Message: "Invalid productId",
			Details: "productId must be an integer (int32)",
		})
		return 0, false
	}
	if id64 < 1 {
		c.JSON(http.StatusBadRequest, ErrorResp{
			Error:   "INVALID_INPUT",
			Message: "Invalid productId",
			Details: "productId must be >= 1",
		})
		return 0, false
	}
	return int32(id64), true
}

// validate Product fields per api.yaml constraints + enforce product_id == path productId
func validateProduct(pathID int32, p Product) *ErrorResp {
	if p.ProductID < 1 {
		return &ErrorResp{"INVALID_INPUT", "Invalid product_id", "product_id must be >= 1"}
	}
	if p.ProductID != pathID {
		return &ErrorResp{"INVALID_INPUT", "product_id mismatch", "product_id must match path productId"}
	}
	if len(p.SKU) < 1 || len(p.SKU) > 100 {
		return &ErrorResp{"INVALID_INPUT", "Invalid sku", "sku length must be 1-100"}
	}
	if len(p.Manufacturer) < 1 || len(p.Manufacturer) > 200 {
		return &ErrorResp{"INVALID_INPUT", "Invalid manufacturer", "manufacturer length must be 1-200"}
	}
	if p.CategoryID < 1 {
		return &ErrorResp{"INVALID_INPUT", "Invalid category_id", "category_id must be >= 1"}
	}
	if p.Weight < 0 {
		return &ErrorResp{"INVALID_INPUT", "Invalid weight", "weight must be >= 0"}
	}
	if p.SomeOtherID < 1 {
		return &ErrorResp{"INVALID_INPUT", "Invalid some_other_id", "some_other_id must be >= 1"}
	}
	return nil
}

func notFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, ErrorResp{
		Error:   "NOT_FOUND",
		Message: msg,
	})
}

// ===== main =====

func main() {
	// Gin router
	r := gin.Default()

	// In-memory store + seed some initial products so POST can return 204 (since spec defines 404 if not found)
	store := NewStore()
	store.Upsert(1, Product{
		ProductID:    1,
		SKU:          "ABC-123-XYZ",
		Manufacturer: "Acme Corporation",
		CategoryID:   456,
		Weight:       1250,
		SomeOtherID:  789,
	})
	store.Upsert(2, Product{
		ProductID:    2,
		SKU:          "DEF-456-QWE",
		Manufacturer: "Example Inc.",
		CategoryID:   100,
		Weight:       0,
		SomeOtherID:  1,
	})

	// ===== Routes required by api.yaml (Products only) =====

	// GET /products/{productId}
	r.GET("/products/:productId", func(c *gin.Context) {
		id, ok := parseProductIDParam(c)
		if !ok {
			return
		}

		p, exists := store.Get(id)
		if !exists {
			notFound(c, "Product not found")
			return
		}

		c.JSON(http.StatusOK, p)
	})

	// POST /products/{productId}/details
	// Success: 204 No Content
	r.POST("/products/:productId/details", func(c *gin.Context) {
		id, ok := parseProductIDParam(c)
		if !ok {
			return
		}

		// Spec: 404 if product not found (so we require it exists before update)
		if _, exists := store.Get(id); !exists {
			notFound(c, "Product not found")
			return
		}

		var p Product
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResp{
				Error:   "INVALID_INPUT",
				Message: "Invalid input data",
				Details: err.Error(),
			})
			return
		}

		if verr := validateProduct(id, p); verr != nil {
			c.JSON(http.StatusBadRequest, verr)
			return
		}

		// Update details
		store.Upsert(id, p)

		// 204: no body
		c.Status(http.StatusNoContent)
	})

	// Start server
	_ = r.Run(":8080")
}
