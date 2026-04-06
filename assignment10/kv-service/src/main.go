package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultLeaderAfterFollowerMessage = 200 * time.Millisecond
	defaultFollowerOnReplicate        = 100 * time.Millisecond
	defaultFollowerOnInternalRead     = 50 * time.Millisecond
	headerKVVersion                   = "X-KV-Version"
)

var (
	leaderAfterFollowerSleep  = defaultLeaderAfterFollowerMessage
	followerReplicateSleep    = defaultFollowerOnReplicate
	followerInternalReadSleep = defaultFollowerOnInternalRead
)

type role string

const (
	roleStandalone role = "standalone"
	roleLeader     role = "leader"
	roleFollower   role = "follower"
	roleLeaderless role = "leaderless"
)

type entry struct {
	Value   string
	Version uint64
}

type Store struct {
	mu    sync.RWMutex
	items map[string]entry
}

func NewStore() *Store {
	return &Store{items: make(map[string]entry)}
}

func (s *Store) Get(key string) (entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.items[key]
	return e, ok
}

func (s *Store) PutIfNewer(key, value string, version uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[key]
	if ok && version <= cur.Version {
		return
	}
	s.items[key] = entry{Value: value, Version: version}
}

func (s *Store) PutLocal(key, value string, version uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = entry{Value: value, Version: version}
}

type setRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type replicateBody struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version uint64 `json:"version"`
}

type internalReadResponse struct {
	OK      bool   `json:"ok"`
	Value   string `json:"value,omitempty"`
	Version uint64 `json:"version,omitempty"`
}

// Config holds N/R/W and cluster topology (from environment).
type Config struct {
	N         int
	R         int
	W         int
	PeerURLs  []string // followers only — leader replicates here
	AllURLs   []string // all N replica base URLs (quorum reads)
	SelfURL   string   // this node's base URL (must match an entry in AllURLs when clustered)
	LeaderURL string   // HTTP base URL of leader (follower R=1 proxy)
}

type Server struct {
	role      role
	cfg       Config
	store     *Store
	client    *http.Client
	versionMu sync.Mutex
	nextVer   uint64
	// lamport is advanced on leaderless writes and replicated updates so versions stay comparable across coordinators.
	lamport uint64
}

func parseRole(s string) role {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "leader":
		return roleLeader
	case "follower":
		return roleFollower
	case "leaderless":
		return roleLeaderless
	default:
		return roleStandalone
	}
}

func parsePeerURLs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		u := strings.TrimSpace(p)
		if u != "" {
			out = append(out, strings.TrimRight(u, "/"))
		}
	}
	return out
}

func envInt(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func loadConfig(r role) Config {
	switch r {
	case roleStandalone:
		return Config{N: 1, R: 1, W: 1}
	case roleLeaderless:
		n := envInt("KV_N", 5)
		all := parsePeerURLs(os.Getenv("KV_ALL_URLS"))
		self := strings.TrimRight(strings.TrimSpace(os.Getenv("KV_SELF_URL")), "/")
		// Part III: W=N, R=1 (fixed for leaderless mode).
		return Config{
			N:       n,
			R:       1,
			W:       n,
			AllURLs: all,
			SelfURL: self,
		}
	default:
		n := envInt("KV_N", 5)
		rVal := envInt("KV_R", 1)
		wVal := envInt("KV_W", 5)
		if rVal > n {
			rVal = n
		}
		if wVal > n {
			wVal = n
		}
		all := parsePeerURLs(os.Getenv("KV_ALL_URLS"))
		if len(all) == 0 {
			// Backward compat: only leader followers listed; reads that need all URLs still work for single-peer tests.
			all = nil
		}
		return Config{
			N:         n,
			R:         rVal,
			W:         wVal,
			PeerURLs:  parsePeerURLs(os.Getenv("KV_PEER_URLS")),
			AllURLs:   all,
			SelfURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("KV_SELF_URL")), "/"),
			LeaderURL: strings.TrimRight(strings.TrimSpace(os.Getenv("KV_LEADER_URL")), "/"),
		}
	}
}

func NewServer(r role, cfg Config) *Server {
	return &Server{
		role:  r,
		cfg:   cfg,
		store: NewStore(),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (srv *Server) nextVersion() uint64 {
	srv.versionMu.Lock()
	defer srv.versionMu.Unlock()
	srv.nextVer++
	return srv.nextVer
}

func (srv *Server) bumpLamport(remote uint64) {
	srv.versionMu.Lock()
	if remote > srv.lamport {
		srv.lamport = remote
	}
	srv.versionMu.Unlock()
}

// nextLeaderlessWriteVersion picks a logical version for a coordinator write (Lamport-style vs local key state).
func (srv *Server) nextLeaderlessWriteVersion(key string) uint64 {
	srv.versionMu.Lock()
	defer srv.versionMu.Unlock()
	srv.lamport++
	v := srv.lamport
	if e, ok := srv.store.Get(key); ok && e.Version >= v {
		v = e.Version + 1
	}
	if v > srv.lamport {
		srv.lamport = v
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (srv *Server) sleepAfterFollowerMessage() {
	if leaderAfterFollowerSleep <= 0 {
		return
	}
	time.Sleep(leaderAfterFollowerSleep)
}

func (srv *Server) postReplicate(ctx context.Context, peerBase, key, value string, version uint64) error {
	body, err := json.Marshal(replicateBody{Key: key, Value: value, Version: version})
	if err != nil {
		return err
	}
	u := peerBase + "/internal/replicate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.client.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// replicateAllFollowersSequential: W=N — every follower must ack; sleep after each message (assignment).
func (srv *Server) replicateAllFollowersSequential(ctx context.Context, key, value string, version uint64) error {
	for _, peer := range srv.cfg.PeerURLs {
		if err := srv.postReplicate(ctx, peer, key, value, version); err != nil {
			return fmt.Errorf("peer %s: %w", peer, err)
		}
		srv.sleepAfterFollowerMessage()
	}
	return nil
}

// replicateToFollowerQuorum: leader local already counts as 1; need W-1 follower acks.
func (srv *Server) replicateToFollowerQuorum(ctx context.Context, key, value string, version uint64) error {
	need := srv.cfg.W - 1
	if need <= 0 {
		return nil
	}
	var ok int32
	peers := append([]string(nil), srv.cfg.PeerURLs...)
	var wg sync.WaitGroup
	for _, peer := range peers {
		p := peer
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := srv.postReplicate(ctx, p, key, value, version); err != nil {
				log.Printf("quorum replicate peer %s: %v", p, err)
				return
			}
			atomic.AddInt32(&ok, 1)
			srv.sleepAfterFollowerMessage()
		}()
	}
	wg.Wait()
	if int(atomic.LoadInt32(&ok)) < need {
		return fmt.Errorf("need %d follower acks, got %d", need, ok)
	}
	return nil
}

// coordinatorReplicateAllOthers: leaderless W=N — every other replica must ack before returning 201.
func (srv *Server) coordinatorReplicateAllOthers(ctx context.Context, key, value string, version uint64) error {
	for _, peer := range srv.cfg.AllURLs {
		b := strings.TrimRight(strings.TrimSpace(peer), "/")
		if b == "" {
			continue
		}
		if srv.cfg.SelfURL != "" && sameBase(b, srv.cfg.SelfURL) {
			continue
		}
		if err := srv.postReplicate(ctx, b, key, value, version); err != nil {
			return fmt.Errorf("peer %s: %w", b, err)
		}
		srv.sleepAfterFollowerMessage()
	}
	return nil
}

func (srv *Server) replicateAllFollowersAsync(key, value string, version uint64) {
	peers := append([]string(nil), srv.cfg.PeerURLs...)
	go func() {
		ctx := context.Background()
		for _, peer := range peers {
			p := peer
			if err := srv.postReplicate(ctx, p, key, value, version); err != nil {
				log.Printf("async replicate %s: %v", p, err)
			} else {
				srv.sleepAfterFollowerMessage()
			}
		}
	}()
}

func (srv *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req setRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Key) == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}

	switch srv.role {
	case roleFollower:
		http.Error(w, "writes must be sent to the leader", http.StatusForbidden)
		return
	case roleLeaderless:
		if len(srv.cfg.AllURLs) == 0 {
			http.Error(w, "KV_ALL_URLS required for leaderless", http.StatusInternalServerError)
			return
		}
		v := srv.nextLeaderlessWriteVersion(req.Key)
		srv.store.PutLocal(req.Key, req.Value, v)
		if err := srv.coordinatorReplicateAllOthers(r.Context(), req.Key, req.Value, v); err != nil {
			log.Printf("leaderless replication error: %v", err)
			http.Error(w, "replication failed", http.StatusBadGateway)
			return
		}
		w.Header().Set(headerKVVersion, strconv.FormatUint(v, 10))
		w.WriteHeader(http.StatusCreated)
	case roleStandalone:
		v := srv.nextVersion()
		srv.store.PutLocal(req.Key, req.Value, v)
		w.Header().Set(headerKVVersion, strconv.FormatUint(v, 10))
		w.WriteHeader(http.StatusCreated)
	case roleLeader:
		v := srv.nextVersion()
		srv.store.PutLocal(req.Key, req.Value, v)
		ctx := r.Context()
		switch {
		case srv.cfg.W >= srv.cfg.N:
			if err := srv.replicateAllFollowersSequential(ctx, req.Key, req.Value, v); err != nil {
				log.Printf("replication error: %v", err)
				http.Error(w, "replication failed", http.StatusBadGateway)
				return
			}
		case srv.cfg.W <= 1:
			srv.replicateAllFollowersAsync(req.Key, req.Value, v)
		default:
			if err := srv.replicateToFollowerQuorum(ctx, req.Key, req.Value, v); err != nil {
				log.Printf("quorum replication error: %v", err)
				http.Error(w, "replication failed", http.StatusBadGateway)
				return
			}
		}
		w.Header().Set(headerKVVersion, strconv.FormatUint(v, 10))
		w.WriteHeader(http.StatusCreated)
	default:
		http.Error(w, "unknown role", http.StatusInternalServerError)
	}
}

func sameBase(a, b string) bool {
	a = strings.TrimRight(strings.TrimSpace(a), "/")
	b = strings.TrimRight(strings.TrimSpace(b), "/")
	return a != "" && b != "" && a == b
}

func (srv *Server) readReplica(ctx context.Context, baseURL, key string) (internalReadResponse, error) {
	if srv.cfg.SelfURL != "" && sameBase(baseURL, srv.cfg.SelfURL) {
		return srv.readLocalInternal(key)
	}
	u := baseURL + "/internal/read?key=" + url.QueryEscape(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return internalReadResponse{}, err
	}
	resp, err := srv.client.Do(req)
	if err != nil {
		return internalReadResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return internalReadResponse{}, fmt.Errorf("internal read status %d", resp.StatusCode)
	}
	var ir internalReadResponse
	if err := json.NewDecoder(resp.Body).Decode(&ir); err != nil {
		return internalReadResponse{}, err
	}
	return ir, nil
}

func (srv *Server) readLocalInternal(key string) (internalReadResponse, error) {
	e, ok := srv.store.Get(key)
	if !ok {
		return internalReadResponse{OK: false}, nil
	}
	return internalReadResponse{OK: true, Value: e.Value, Version: e.Version}, nil
}

func (srv *Server) quorumPick(ctx context.Context, key string) (entry, error) {
	urls := srv.cfg.AllURLs
	if len(urls) == 0 {
		return entry{}, fmt.Errorf("KV_ALL_URLS not configured for quorum read")
	}

	type result struct {
		ir internalReadResponse
	}
	var mu sync.Mutex
	var collected []internalReadResponse
	var wg sync.WaitGroup
	for _, bu := range urls {
		b := strings.TrimRight(strings.TrimSpace(bu), "/")
		if b == "" {
			continue
		}
		wg.Add(1)
		go func(base string) {
			defer wg.Done()
			ir, err := srv.readReplica(ctx, base, key)
			if err != nil {
				log.Printf("read replica %s: %v", base, err)
				return
			}
			mu.Lock()
			collected = append(collected, ir)
			mu.Unlock()
		}(b)
	}
	wg.Wait()

	if len(collected) < srv.cfg.R {
		return entry{}, fmt.Errorf("need %d read acks, got %d", srv.cfg.R, len(collected))
	}

	var best *internalReadResponse
	for i := range collected {
		ir := &collected[i]
		if !ir.OK {
			continue
		}
		if best == nil || ir.Version > best.Version {
			best = ir
		}
	}
	if best == nil {
		return entry{}, errNotFound
	}
	return entry{Value: best.Value, Version: best.Version}, nil
}

var errNotFound = errors.New("not found")

func (srv *Server) proxyGetToLeader(w http.ResponseWriter, key string) {
	if srv.cfg.LeaderURL == "" {
		http.Error(w, "KV_LEADER_URL not set", http.StatusInternalServerError)
		return
	}
	u := srv.cfg.LeaderURL + "/get?key=" + url.QueryEscape(key)
	resp, err := srv.client.Get(u)
	if err != nil {
		http.Error(w, "leader unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header, headerKVVersion)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header, keys ...string) {
	for _, k := range keys {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
}

func (srv *Server) getLocalPlain(w http.ResponseWriter, key string) {
	e, ok := srv.store.Get(key)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set(headerKVVersion, strconv.FormatUint(e.Version, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(e.Value))
}

func (srv *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	switch srv.role {
	case roleStandalone:
		srv.getLocalPlain(w, key)
		return
	case roleLeaderless:
		// R=1: return this node's value only (stale reads possible before a concurrent write finishes).
		srv.getLocalPlain(w, key)
		return
	case roleFollower:
		if srv.cfg.R == 1 {
			srv.proxyGetToLeader(w, key)
			return
		}
		srv.handleQuorumGet(w, r, key)
		return
	case roleLeader:
		if srv.cfg.R == 1 {
			srv.getLocalPlain(w, key)
			return
		}
		srv.handleQuorumGet(w, r, key)
		return
	default:
		http.Error(w, "unknown role", http.StatusInternalServerError)
	}
}

func (srv *Server) handleQuorumGet(w http.ResponseWriter, r *http.Request, key string) {
	e, err := srv.quorumPick(r.Context(), key)
	if err != nil {
		if errors.Is(err, errNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, "quorum read failed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set(headerKVVersion, strconv.FormatUint(e.Version, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(e.Value))
}

func (srv *Server) handleInternalReplicate(w http.ResponseWriter, r *http.Request) {
	if srv.role != roleFollower && srv.role != roleStandalone && srv.role != roleLeaderless {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var rb replicateBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&rb); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(rb.Key) == "" {
		http.Error(w, "key cannot be empty", http.StatusBadRequest)
		return
	}
	time.Sleep(followerReplicateSleep)
	srv.store.PutIfNewer(rb.Key, rb.Value, rb.Version)
	if srv.role == roleLeaderless {
		srv.bumpLamport(rb.Version)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *Server) handleInternalRead(w http.ResponseWriter, r *http.Request) {
	if srv.role != roleFollower && srv.role != roleLeader && srv.role != roleStandalone && srv.role != roleLeaderless {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	if (srv.role == roleFollower || srv.role == roleLeaderless) && followerInternalReadSleep > 0 {
		time.Sleep(followerInternalReadSleep)
	}
	// Leader (and standalone): no sleep per assignment for leader read path.
	e, ok := srv.store.Get(key)
	if !ok {
		writeJSON(w, http.StatusOK, internalReadResponse{OK: false})
		return
	}
	writeJSON(w, http.StatusOK, internalReadResponse{OK: true, Value: e.Value, Version: e.Version})
}

type configResponse struct {
	N         int      `json:"n"`
	R         int      `json:"r"`
	W         int      `json:"w"`
	Role      string   `json:"role"`
	Topology  string   `json:"topology,omitempty"` // "leader_follower" | "leaderless"
	LeaderURL string   `json:"leader_url,omitempty"`
	SelfURL   string   `json:"self_url,omitempty"`
	PeerCount int      `json:"peer_count"`
	AllURLs   []string `json:"all_urls,omitempty"`
}

func (srv *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	allCopy := append([]string(nil), srv.cfg.AllURLs...)
	sort.Strings(allCopy)
	peerCount := len(srv.cfg.PeerURLs)
	topology := "leader_follower"
	if srv.role == roleLeaderless {
		topology = "leaderless"
		peerCount = 0
		for _, u := range srv.cfg.AllURLs {
			b := strings.TrimRight(strings.TrimSpace(u), "/")
			if b == "" || (srv.cfg.SelfURL != "" && sameBase(b, srv.cfg.SelfURL)) {
				continue
			}
			peerCount++
		}
	}
	resp := configResponse{
		N:         srv.cfg.N,
		R:         srv.cfg.R,
		W:         srv.cfg.W,
		Role:      string(srv.role),
		Topology:  topology,
		LeaderURL: srv.cfg.LeaderURL,
		SelfURL:   srv.cfg.SelfURL,
		PeerCount: peerCount,
		AllURLs:   allCopy,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleLocalRead exposes raw local store state for consistency tests (no proxy, no quorum).
// Same JSON shape as /internal/read but intended as a "sneaky" public test hook.
func (srv *Server) handleLocalRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}
	e, ok := srv.store.Get(key)
	if !ok {
		writeJSON(w, http.StatusOK, internalReadResponse{OK: false})
		return
	}
	writeJSON(w, http.StatusOK, internalReadResponse{OK: true, Value: e.Value, Version: e.Version})
}

func (srv *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/set", srv.handleSet)
	mux.HandleFunc("/get", srv.handleGet)
	mux.HandleFunc("/config", srv.handleConfig)
	mux.HandleFunc("/local_read", srv.handleLocalRead)
	mux.HandleFunc("/internal/replicate", srv.handleInternalReplicate)
	mux.HandleFunc("/internal/read", srv.handleInternalRead)
	mux.HandleFunc("/health", srv.handleHealth)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r := parseRole(os.Getenv("KV_ROLE"))
	cfg := loadConfig(r)

	if r == roleLeader && len(cfg.PeerURLs) == 0 {
		log.Println("warning: KV_ROLE=leader but KV_PEER_URLS is empty")
	}
	if (r == roleLeader || r == roleFollower) && cfg.R > 1 && len(cfg.AllURLs) == 0 {
		log.Println("warning: quorum read (R>1) needs KV_ALL_URLS listing all replica base URLs")
	}
	if r == roleFollower && cfg.R == 1 && cfg.LeaderURL == "" {
		log.Println("warning: follower R=1 expects KV_LEADER_URL for /get forwarding")
	}
	if r == roleLeaderless {
		if len(cfg.AllURLs) == 0 {
			log.Println("warning: leaderless mode needs KV_ALL_URLS (all replica base URLs)")
		}
		if cfg.SelfURL == "" {
			log.Println("warning: leaderless should set KV_SELF_URL to skip self in replication fan-out")
		}
	}

	srv := NewServer(r, cfg)
	mux := http.NewServeMux()
	srv.Register(mux)

	addr := ":" + port
	log.Printf("kv-service %s N=%d R=%d W=%d peers=%d all=%d", addr, cfg.N, cfg.R, cfg.W, len(cfg.PeerURLs), len(cfg.AllURLs))
	if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
