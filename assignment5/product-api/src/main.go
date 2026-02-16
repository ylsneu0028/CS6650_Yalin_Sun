package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// ---- OpenAPI Schemas ----

type Product struct {
	ProductID    int32  `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int32  `json:"category_id"`
	Weight       int32  `json:"weight"`
	SomeOtherID  int32  `json:"some_other_id"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ---- In-memory store ----

type Store struct {
	mu       sync.RWMutex
	products map[int32]Product
}

func NewStore() *Store {
	s := &Store{
		products: make(map[int32]Product),
	}
	// Seed some products so GET/POST can demonstrate behavior.
	s.products[1] = Product{
		ProductID:    1,
		SKU:          "ABC-123-XYZ",
		Manufacturer: "Acme Corporation",
		CategoryID:   456,
		Weight:       1250,
		SomeOtherID:  789,
	}
	s.products[2] = Product{
		ProductID:    2,
		SKU:          "SKU-0002",
		Manufacturer: "Contoso",
		CategoryID:   10,
		Weight:       300,
		SomeOtherID:  999,
	}
		s.products[3] = Product{
		ProductID:    3,
		SKU:          "SKU-0003",
		Manufacturer: "SeedCo",
		CategoryID:   1,
		Weight:       1200,
		SomeOtherID:  103,
	}

	s.products[4] = Product{
		ProductID:    4,
		SKU:          "SKU-0004",
		Manufacturer: "SeedCo",
		CategoryID:   1,
		Weight:       1300,
		SomeOtherID:  104,
	}

	s.products[5] = Product{
		ProductID:    5,
		SKU:          "SKU-0005",
		Manufacturer: "SeedCo",
		CategoryID:   1,
		Weight:       1400,
		SomeOtherID:  105,
	}
	
	return s
}

func (s *Store) Get(id int32) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

func (s *Store) Update(id int32, p Product) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[id]; !ok {
		return false
	}
	s.products[id] = p
	return true
}

// ---- Validation helpers ----

func parseProductID(c *gin.Context) (int32, bool) {
	raw := c.Param("productId")
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, false
	}
	return int32(v), true
}

func validateProductBody(p Product) (ok bool, errResp ErrorResponse) {
	// required fields: product_id, sku, manufacturer, category_id, weight, some_other_id
	if p.ProductID < 1 {
		return false, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "product_id must be a positive integer",
		}
	}
	if len(strings.TrimSpace(p.SKU)) < 1 || len(p.SKU) > 100 {
		return false, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "sku length must be between 1 and 100",
		}
	}
	if len(strings.TrimSpace(p.Manufacturer)) < 1 || len(p.Manufacturer) > 200 {
		return false, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "manufacturer length must be between 1 and 200",
		}
	}
	if p.CategoryID < 1 {
		return false, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "category_id must be a positive integer",
		}
	}
	if p.Weight < 0 {
		return false, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "weight must be >= 0",
		}
	}
	if p.SomeOtherID < 1 {
		return false, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: "some_other_id must be a positive integer",
		}
	}
	return true, ErrorResponse{}
}

// ---- Auth middleware (ApiKeyAuth OR BearerAuth) ----

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
		auth := strings.TrimSpace(c.GetHeader("Authorization"))

		hasAPIKey := apiKey != ""
		hasBearer := strings.HasPrefix(strings.ToLower(auth), "bearer ") && len(strings.TrimSpace(auth[7:])) > 0

		if !hasAPIKey && !hasBearer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "UNAUTHORIZED",
				Message: "Missing authentication",
				Details: "Provide X-API-Key header or Authorization: Bearer <token>",
			})
			return
		}
		c.Next()
	}
}

// ---- Handlers ----

func getProductHandler(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseProductID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "productId must be a positive integer",
			})
			return
		}

		p, found := store.Get(id)
		if !found {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
				Details: "No product exists with the given productId",
			})
			return
		}

		c.JSON(http.StatusOK, p)
	}
}

func addProductDetailsHandler(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		pathID, ok := parseProductID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "productId must be a positive integer",
			})
			return
		}

		var body Product
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "Request body must be valid JSON matching Product schema",
			})
			return
		}

		if ok, errResp := validateProductBody(body); !ok {
			c.JSON(http.StatusBadRequest, errResp)
			return
		}

		// Ensure path ID and body product_id are consistent (good practice)
		if body.ProductID != pathID {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "product_id in body must match productId in path",
			})
			return
		}

		updated := store.Update(pathID, body)
		if !updated {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
				Details: "No product exists with the given productId",
			})
			return
		}

		c.Status(http.StatusNoContent) // 204
	}
}

func main() {
	store := NewStore()

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Apply auth globally (matches OpenAPI global security concept)
	r.Use(authMiddleware())

	// Routes (Product API part only)
	r.GET("/products/:productId", getProductHandler(store))
	r.POST("/products/:productId/details", addProductDetailsHandler(store))

	// Run
	_ = r.Run(":8080")
}

