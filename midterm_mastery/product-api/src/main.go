package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net/http"
	"os"
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

// -------------------- Fake Downstream (fault injection) --------------------

func startFakeDownstream() {
	mode := os.Getenv("DOWNSTREAM_MODE") // ok | slow | hang | flaky
	mux := http.NewServeMux()

	mux.HandleFunc("/reco", func(w http.ResponseWriter, r *http.Request) {
		switch mode {
		case "slow":
			time.Sleep(5 * time.Second)
			w.WriteHeader(200)
			w.Write([]byte("slow-ok"))
		case "hang":
			select {} // never returns
		case "flaky":
			if rand.Intn(10) < 3 {
				w.WriteHeader(500)
				w.Write([]byte("fail"))
				return
			}
			time.Sleep(2 * time.Second)
			w.WriteHeader(200)
			w.Write([]byte("flaky-ok"))
		default:
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(200)
			w.Write([]byte("ok"))
		}
	})

	srv := &http.Server{Addr: ":9090", Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
}

// -------------------- Resilience: Bulkhead + Circuit Breaker --------------------

type Bulkhead struct{ sem chan struct{} }

func NewBulkhead(n int) *Bulkhead { return &Bulkhead{sem: make(chan struct{}, n)} }

func (b *Bulkhead) TryAcquire() bool {
	select {
	case b.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (b *Bulkhead) Release() { <-b.sem }

type BreakerState int

const (
	Closed BreakerState = iota
	Open
	HalfOpen
)

func (s BreakerState) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

type CircuitBreaker struct {
	mu                   sync.Mutex
	state                BreakerState
	consecutiveFails     int
	openUntil            time.Time
	failThreshold        int
	openFor              time.Duration
	halfOpenProbeInFlight bool
}

func NewCircuitBreaker(threshold int, openFor time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:         Closed,
		failThreshold: threshold,
		openFor:       openFor,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	if cb.state == Open {
		if now.Before(cb.openUntil) {
			return false
		}
		// transition to half-open after open window
		cb.state = HalfOpen
		cb.halfOpenProbeInFlight = false
	}

	if cb.state == HalfOpen {
		if cb.halfOpenProbeInFlight {
			return false // allow only one probe request at a time
		}
		cb.halfOpenProbeInFlight = true
		return true
	}

	// closed
	return true
}

func (cb *CircuitBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = Closed
	cb.consecutiveFails = 0
	cb.halfOpenProbeInFlight = false
}

func (cb *CircuitBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails++
	cb.halfOpenProbeInFlight = false

	if cb.consecutiveFails >= cb.failThreshold {
		cb.state = Open
		cb.openUntil = time.Now().Add(cb.openFor)
	}
}

func (cb *CircuitBreaker) Snapshot() (state BreakerState, fails int, openUntil time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state, cb.consecutiveFails, cb.openUntil
}

// -------------------- Downstream Client (fail-fast + bulkhead + breaker) --------------------

type RecoClient struct {
	baseURL string
	client  *http.Client
	bh      *Bulkhead
	cb      *CircuitBreaker
}

func NewRecoClient(baseURL string) *RecoClient {
	return &RecoClient{
		baseURL: baseURL,
		// Fail-fast: never allow downstream calls to hang forever.
		client: &http.Client{Timeout: 120 * time.Millisecond},
		// Bulkhead: cap downstream concurrency so it can't starve the whole service.
		bh: NewBulkhead(20),
		// Circuit breaker: stop hammering a broken dependency.
		cb: NewCircuitBreaker(10, 10*time.Second),
	}
}

// GetReco is intentionally "best-effort": it returns quickly and degrades under failure.
// We don't need the actual reco payload for this assignment demo; we just need the dependency behavior.
func (rc *RecoClient) GetReco(ctx context.Context, pid int) (string, error) {
	// Circuit breaker gate
	if !rc.cb.Allow() {
		return "degraded(circuit-open)", nil
	}

	// Bulkhead gate
	if !rc.bh.TryAcquire() {
		rc.cb.OnFailure()
		return "degraded(bulkhead-full)", nil
	}
	defer rc.bh.Release()

	// Per-call timeout (even tighter than client timeout)
	callCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()

	url := fmt.Sprintf("%s/reco?pid=%d", rc.baseURL, pid)
	req, _ := http.NewRequestWithContext(callCtx, "GET", url, nil)

	resp, err := rc.client.Do(req)
	if err != nil {
		rc.cb.OnFailure()
		return "degraded(timeout-or-neterr)", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rc.cb.OnFailure()
		return "degraded(downstream-non200)", nil
	}

	rc.cb.OnSuccess()
	return "ok", nil
}

// -------------------- HTTP Server --------------------

func main() {
	rand.Seed(time.Now().UnixNano())
	startFakeDownstream()

	store := NewStore(100000)

	// Mode switch for your demo:
	// - RESILIENCE_MODE=problem -> no timeouts / no bulkhead / no breaker (shows failure)
	// - RESILIENCE_MODE=fix     -> fail-fast + bulkhead + circuit breaker (shows recovery)
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("RESILIENCE_MODE")))
	if mode == "" {
		mode = "fix" // default to safe behavior
	}

	// Shared resilience client (used only in "fix" mode)
	reco := NewRecoClient("http://127.0.0.1:9090")

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Public health endpoint for ALB
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Debug stats to show breaker/bulkhead behavior during demo
	r.GET("/debug/stats", func(c *gin.Context) {
		state, fails, until := reco.cb.Snapshot()
		c.JSON(http.StatusOK, gin.H{
			"resilience_mode":     mode,
			"downstream_mode":     os.Getenv("DOWNSTREAM_MODE"),
			"breaker_state":       state.String(),
			"consecutive_fails":   fails,
			"open_until":          until,
			"bulkhead_capacity":   cap(reco.bh.sem),
			"bulkhead_inflight":   len(reco.bh.sem),
			"downstream_base_url": reco.baseURL,
		})
	})

	// Protected routes
	auth := r.Group("/")
	auth.Use(authMiddleware())

	auth.GET("/products/search", func(c *gin.Context) {
		q := c.Query("q")
		resp := store.Search(q)

		// --- Dependency call injected for Assignment 7 crash/recovery demo ---
		// Keep the Assignment 6 "fixed compute" search logic unchanged.
		// We only add a simulated dependency to show cascading failures and resiliency patterns.
		if mode == "problem" {
			// Problem mode: intentionally dangerous (no timeout).
			// Under DOWNSTREAM_MODE=slow/hang, these calls will pile up and degrade the service.
			client := &http.Client{} // Timeout=0 => can hang forever
			for i := range resp.Products {
				url := fmt.Sprintf("http://127.0.0.1:9090/reco?pid=%d", resp.Products[i].ID)
				req, _ := http.NewRequest("GET", url, nil)
				_, _ = client.Do(req) // ignore errors intentionally
			}
		} else {
			// Fix mode: fail-fast + bulkhead + circuit breaker
			for i := range resp.Products {
				_, _ = reco.GetReco(c.Request.Context(), resp.Products[i].ID)
			}
		}

		c.JSON(http.StatusOK, resp)
	})

	_ = r.Run(":8080")
}