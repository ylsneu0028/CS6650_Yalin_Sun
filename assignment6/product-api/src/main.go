package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// -------------------- Data Model --------------------

type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

type SearchResponse struct {
	Products   []Product `json:"products"`     // Max 20 results
	TotalFound int       `json:"total_found"`  // Total matches found (within checked set)
	Checked    int       `json:"checked"`      // Evidence: should ALWAYS be 100
	SearchTime string    `json:"search_time"`  // Optional
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// -------------------- Store (sync.Map) --------------------

type Store struct {
	products sync.Map // key: int, value: Product
	ids      []int    // stable iteration order: 1..N
	size     int
}

func NewStore(n int) *Store {
	s := &Store{
		ids:  make([]int, n),
		size: n,
	}

	brands := []string{"Alpha", "Beta", "Gamma", "Delta", "Omega", "Nova", "Zen", "Apex"}
	categories := []string{"Electronics", "Books", "Home", "Sports", "Clothing", "Beauty", "Toys", "Grocery"}

	for i := 1; i <= n; i++ {
		brand := brands[i%len(brands)]
		category := categories[i%len(categories)]

		p := Product{
			ID:          i,
			Name:        fmt.Sprintf("Product %s %d", brand, i),
			Category:    category,
			Brand:       brand,
			Description: fmt.Sprintf("Description for %s item %d in %s category.", brand, i, category),
		}

		s.products.Store(i, p)
		s.ids[i-1] = i
	}
	return s
}

func hash32(s string) uint32 {
	var h uint32 = 2166136261 // FNV-1a
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// -------------------- Fixed Compute Work --------------------

const computeRounds = 80

func fixedComputeWork(input string) {
	// chain hashing to create bounded-but-nontrivial CPU work
	data := []byte(input)
	var sum [32]byte
	for i := 0; i < computeRounds; i++ {
		sum = sha256.Sum256(data)
		data = sum[:] // next round input
	}
	_ = sum[0]
}

// -------------------- Auth Middleware (from assignment5 style) --------------------

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
		auth := strings.TrimSpace(c.GetHeader("Authorization"))

		hasAPIKey := apiKey != ""
		hasBearer := strings.HasPrefix(strings.ToLower(auth), "bearer ") &&
			len(strings.TrimSpace(auth[7:])) > 0

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

// -------------------- Search Logic --------------------

func (s *Store) Search(query string) SearchResponse {
	start := time.Now()

	q := strings.ToLower(strings.TrimSpace(query))
	startIdx := int(hash32(q) % uint32(s.size))

	const toCheck = 100
	const maxReturn = 20

	results := make([]Product, 0, maxReturn)
	totalFound := 0
	checked := 0

	for i := 0; i < toCheck; i++ {
		id := s.ids[(startIdx+i)%s.size]

		v, ok := s.products.Load(id)
		checked++
		if !ok {
			continue
		}
		p := v.(Product)

		// fixed compute per checked product
		fixedComputeWork(p.Name + "|" + p.Category)

		if q != "" {
			// (minor micro-opt: compute lower once)
			nameLower := strings.ToLower(p.Name)
			catLower := strings.ToLower(p.Category)

			if strings.Contains(nameLower, q) || strings.Contains(catLower, q) {
				totalFound++
				if len(results) < maxReturn {
					results = append(results, p)
				}
			}
		}
	}

	return SearchResponse{
		Products:   results,
		TotalFound: totalFound,
		Checked:    checked, // should be 100
		SearchTime: time.Since(start).String(),
	}
}

// -------------------- HTTP Server --------------------

func main() {
    store := NewStore(100000)

    r := gin.New()
    r.Use(gin.Logger(), gin.Recovery())

    // Public health endpoint for ALB
    r.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"ok": true})
    })

    // Protected routes
    auth := r.Group("/")
    auth.Use(authMiddleware())

    auth.GET("/products/search", func(c *gin.Context) {
        q := c.Query("q")
        resp := store.Search(q)
        c.JSON(http.StatusOK, resp)
    })

    // auth.GET("/products/:productId", getProductHandler)
    // auth.POST("/products/:productId/details", addProductDetailsHandler)

    _ = r.Run(":8080")
}