package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin" // Go web framework
)

// --- Data Models ---

// Product is the main data object for our API
type Product struct {
	ProductID    int32  `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int32  `json:"category_id"`
	Weight       int32  `json:"weight"`
	SomeOtherID  int32  `json:"some_other_id"`
}

// ErrorResponse is a standard format for returning errors
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// --- In-Memory Store (simulates a database) ---

// Store holds products in a map, with a lock for safe concurrent access
type Store struct {
	mu       sync.RWMutex      // Lock: allows many readers or one writer at a time
	products map[int32]Product // product_id -> Product
}

// NewStore creates the store and loads some sample products
func NewStore() *Store {
	s := &Store{
		products: make(map[int32]Product),
	}
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

// Get finds a product by ID (read lock — doesn't block other readers)
func (s *Store) Get(id int32) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

// Update replaces a product's data (write lock — blocks everyone else)
// Returns false if product doesn't exist
func (s *Store) Update(id int32, p Product) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[id]; !ok {
		return false
	}
	s.products[id] = p
	return true
}

// --- Validation ---

// parseProductID gets the product ID from the URL and checks it's valid (>= 1)
func parseProductID(c *gin.Context) (int32, bool) {
	raw := c.Param("productId")
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, false
	}
	return int32(v), true
}

// validateProductBody checks that all required fields are valid
func validateProductBody(p Product) (ok bool, errResp ErrorResponse) {
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

// --- Auth Middleware ---
// Runs before every request. Checks for API key or Bearer token.
// If neither is provided, returns 401 and blocks the request.

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
		c.Next() // Auth passed, continue to handler
	}
}

// --- Handlers ---

// GET /products/:productId — returns a product by ID (CRUD: Read)
func getProductHandler(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate ID from URL
		id, ok := parseProductID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "productId must be a positive integer",
			})
			return
		}

		// Look up product
		p, found := store.Get(id)
		if !found {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
				Details: "No product exists with the given productId",
			})
			return
		}

		// Return product as JSON
		c.JSON(http.StatusOK, p)
	}
}

// POST /products/:productId/details — updates a product's details (CRUD: Update)
func addProductDetailsHandler(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate ID from URL
		pathID, ok := parseProductID(c)
		if !ok {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "productId must be a positive integer",
			})
			return
		}

		// Parse JSON body into Product
		var body Product
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "Request body must be valid JSON matching Product schema",
			})
			return
		}

		// Validate all fields
		if ok, errResp := validateProductBody(body); !ok {
			c.JSON(http.StatusBadRequest, errResp)
			return
		}

		// Make sure URL id and body id match
		if body.ProductID != pathID {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "INVALID_INPUT",
				Message: "The provided input data is invalid",
				Details: "product_id in body must match productId in path",
			})
			return
		}

		// Update the product
		updated := store.Update(pathID, body)
		if !updated {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "NOT_FOUND",
				Message: "Product not found",
				Details: "No product exists with the given productId",
			})
			return
		}

		c.Status(http.StatusNoContent) // 204 — success, no body returned
	}
}

// --- Main: starts the server ---
// This service is stateless — no session data stored between requests.
// That means it can be horizontally scaled behind a load balancer.

func main() {
	store := NewStore()

	db, err := openShoppingDB()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(authMiddleware())

	r.GET("/products/:productId", getProductHandler(store))
	r.POST("/products/:productId/details", addProductDetailsHandler(store))

	r.POST("/shopping-carts", postShoppingCart(db))
	r.GET("/shopping-carts/:shoppingCartId", getShoppingCart(db))
	r.POST("/shopping-carts/:shoppingCartId/items", postCartItems(db, store))
	r.DELETE("/shopping-carts/:shoppingCartId/items/:productId", deleteCartItem(db))

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
