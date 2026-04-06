package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	leaderAfterFollowerSleep = 0
	followerReplicateSleep = 0
	followerInternalReadSleep = 0
	os.Exit(m.Run())
}

func TestStandaloneSetGet(t *testing.T) {
	srv := NewServer(roleStandalone, Config{N: 1, R: 1, W: 1})
	mux := http.NewServeMux()
	srv.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := `{"key":"a","value":"hello"}`
	res, err := http.Post(ts.URL+"/set", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("set: %s", res.Status)
	}

	g, err := http.Get(ts.URL + "/get?key=a")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Body.Close()
	if g.StatusCode != http.StatusOK {
		t.Fatalf("get: %s", g.Status)
	}
	b, _ := io.ReadAll(g.Body)
	if string(b) != "hello" {
		t.Fatalf("value=%q", b)
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	srv := NewServer(roleStandalone, Config{N: 1, R: 1, W: 1})
	mux := http.NewServeMux()
	srv.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := `{"key":"","value":"x"}`
	res, err := http.Post(ts.URL+"/set", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 got %s", res.Status)
	}
}

func TestEmptyValueAllowed(t *testing.T) {
	srv := NewServer(roleStandalone, Config{N: 1, R: 1, W: 1})
	mux := http.NewServeMux()
	srv.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := `{"key":"k","value":""}`
	res, err := http.Post(ts.URL+"/set", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("set: %s", res.Status)
	}
	g, _ := http.Get(ts.URL + "/get?key=k")
	defer g.Body.Close()
	if g.StatusCode != http.StatusOK {
		t.Fatalf("get: %s", g.Status)
	}
	b, _ := io.ReadAll(g.Body)
	if string(b) != "" {
		t.Fatalf("want empty body, got %q", b)
	}
}

func TestFollowerRejectsClientWrite(t *testing.T) {
	srv := NewServer(roleFollower, Config{N: 5, R: 1, W: 5})
	mux := http.NewServeMux()
	srv.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/set", "application/json", bytes.NewReader([]byte(`{"key":"a","value":"b"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403 got %s", res.Status)
	}
}

func TestLeaderReplicatesToFollower(t *testing.T) {
	follow := NewServer(roleFollower, Config{N: 2, R: 1, W: 2, PeerURLs: nil})
	fmux := http.NewServeMux()
	follow.Register(fmux)
	fts := httptest.NewServer(fmux)
	defer fts.Close()

	lead := NewServer(roleLeader, Config{
		N:        2,
		R:        1,
		W:        2,
		PeerURLs: []string{fts.URL},
	})
	lmux := http.NewServeMux()
	lead.Register(lmux)
	lts := httptest.NewServer(lmux)
	defer lts.Close()

	res, err := http.Post(lts.URL+"/set", "application/json", bytes.NewReader([]byte(`{"key":"x","value":"v1"}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("leader set: %s", res.Status)
	}

	ir, err := http.Get(fts.URL + "/internal/read?key=x")
	if err != nil {
		t.Fatal(err)
	}
	defer ir.Body.Close()
	var body internalReadResponse
	if err := json.NewDecoder(ir.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.Value != "v1" {
		t.Fatalf("follower internal read %+v", body)
	}
}

func TestInternalRead(t *testing.T) {
	srv := NewServer(roleFollower, Config{N: 5, R: 1, W: 5})
	srv.store.PutLocal("k", "v", 3)
	mux := http.NewServeMux()
	srv.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/internal/read?key=k")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %s", res.Status)
	}
	var ir internalReadResponse
	if err := json.NewDecoder(res.Body).Decode(&ir); err != nil {
		t.Fatal(err)
	}
	if !ir.OK || ir.Value != "v" || ir.Version != 3 {
		t.Fatalf("bad response %+v", ir)
	}
}

func TestConfigEndpoint(t *testing.T) {
	srv := NewServer(roleLeader, Config{N: 5, R: 3, W: 3, PeerURLs: []string{"http://a:1"}, AllURLs: []string{"http://l:8080"}, SelfURL: "http://l:8080", LeaderURL: "http://l:8080"})
	mux := http.NewServeMux()
	srv.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/config")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var c configResponse
	if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
		t.Fatal(err)
	}
	if c.N != 5 || c.R != 3 || c.W != 3 || c.Role != "leader" || c.PeerCount != 1 {
		t.Fatalf("config %+v", c)
	}
}

func TestQuorumReadPicksHighestVersion(t *testing.T) {
	urls := make([]string, 2)

	s2 := NewServer(roleFollower, Config{N: 2, R: 2, W: 2, AllURLs: urls})
	s2.store.PutLocal("k", "winner", 10)
	mux2 := http.NewServeMux()
	s2.Register(mux2)
	ts2 := httptest.NewServer(mux2)
	defer ts2.Close()
	urls[1] = ts2.URL

	s1 := NewServer(roleLeader, Config{N: 2, R: 2, W: 2, AllURLs: urls})
	s1.store.PutLocal("k", "loser", 1)
	mux1 := http.NewServeMux()
	s1.Register(mux1)
	ts1 := httptest.NewServer(mux1)
	defer ts1.Close()
	urls[0] = ts1.URL
	s1.cfg.SelfURL = ts1.URL
	s2.cfg.SelfURL = ts2.URL

	g, err := http.Get(ts1.URL + "/get?key=k")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Body.Close()
	if g.StatusCode != http.StatusOK {
		t.Fatalf("get: %s", g.Status)
	}
	b, _ := io.ReadAll(g.Body)
	if string(b) != "winner" {
		t.Fatalf("want winner, got %q", b)
	}
}

func TestFollowerProxiesR1ToLeader(t *testing.T) {
	lead := NewServer(roleLeader, Config{N: 5, R: 1, W: 5})
	lead.store.PutLocal("k", "from-leader", 7)
	lmux := http.NewServeMux()
	lead.Register(lmux)
	lts := httptest.NewServer(lmux)
	defer lts.Close()

	follow := NewServer(roleFollower, Config{
		N:         5,
		R:         1,
		W:         5,
		LeaderURL: lts.URL,
	})
	fmux := http.NewServeMux()
	follow.Register(fmux)
	fts := httptest.NewServer(fmux)
	defer fts.Close()

	g, err := http.Get(fts.URL + "/get?key=k")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Body.Close()
	if g.StatusCode != http.StatusOK {
		t.Fatalf("get: %s", g.Status)
	}
	b, _ := io.ReadAll(g.Body)
	if string(b) != "from-leader" {
		t.Fatalf("body=%q", b)
	}
	if g.Header.Get(headerKVVersion) != "7" {
		t.Fatalf("version header %q", g.Header.Get(headerKVVersion))
	}
}

func TestW1ReturnsBeforeAsyncReplication(t *testing.T) {
	follow := NewServer(roleFollower, Config{N: 2, R: 1, W: 1})
	fmux := http.NewServeMux()
	follow.Register(fmux)
	fts := httptest.NewServer(fmux)
	defer fts.Close()

	lead := NewServer(roleLeader, Config{
		N:        2,
		R:        1,
		W:        1,
		PeerURLs: []string{fts.URL},
	})
	lmux := http.NewServeMux()
	lead.Register(lmux)
	lts := httptest.NewServer(lmux)
	defer lts.Close()

	res, err := http.Post(lts.URL+"/set", "application/json", bytes.NewReader([]byte(`{"key":"x","value":"fast"}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("set: %s", res.Status)
	}
	// Async path may not have landed yet; leader must see write immediately.
	g, _ := http.Get(lts.URL + "/get?key=x")
	defer g.Body.Close()
	if g.StatusCode != http.StatusOK {
		t.Fatalf("leader get: %s", g.Status)
	}
}

func TestLeaderlessCoordinatorReplicatesToPeers(t *testing.T) {
	urls := make([]string, 2)

	b := NewServer(roleLeaderless, Config{N: 2, R: 1, W: 2, AllURLs: urls})
	bmux := http.NewServeMux()
	b.Register(bmux)
	tsB := httptest.NewServer(bmux)
	defer tsB.Close()
	urls[1] = tsB.URL

	a := NewServer(roleLeaderless, Config{N: 2, R: 1, W: 2, AllURLs: urls})
	amux := http.NewServeMux()
	a.Register(amux)
	tsA := httptest.NewServer(amux)
	defer tsA.Close()
	urls[0] = tsA.URL
	a.cfg.SelfURL = tsA.URL
	b.cfg.SelfURL = tsB.URL

	res, err := http.Post(tsA.URL+"/set", "application/json", bytes.NewReader([]byte(`{"key":"k","value":"coord"}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("set: %s", res.Status)
	}

	ir, err := http.Get(tsB.URL + "/internal/read?key=k")
	if err != nil {
		t.Fatal(err)
	}
	defer ir.Body.Close()
	var body internalReadResponse
	if err := json.NewDecoder(ir.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.Value != "coord" {
		t.Fatalf("peer did not receive write: %+v", body)
	}
}

func TestLeaderlessReadIsLocalOnly(t *testing.T) {
	s1 := NewServer(roleLeaderless, Config{N: 2, R: 1, W: 2, AllURLs: []string{"http://a", "http://b"}, SelfURL: "http://a"})
	s1.store.PutLocal("x", "only-here", 1)
	mux := http.NewServeMux()
	s1.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	s2 := NewServer(roleLeaderless, Config{N: 2, R: 1, W: 2, AllURLs: []string{"http://a", ts.URL}, SelfURL: ts.URL})
	mux2 := http.NewServeMux()
	s2.Register(mux2)
	ts2 := httptest.NewServer(mux2)
	defer ts2.Close()

	g, _ := http.Get(ts2.URL + "/get?key=x")
	defer g.Body.Close()
	if g.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 local read on empty node, got %s", g.Status)
	}
}

func TestLeaderlessConfigShowsTopology(t *testing.T) {
	s := NewServer(roleLeaderless, Config{N: 5, R: 1, W: 5, AllURLs: []string{"http://a:1", "http://b:2"}, SelfURL: "http://a:1"})
	mux := http.NewServeMux()
	s.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/config")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var c configResponse
	if err := json.NewDecoder(res.Body).Decode(&c); err != nil {
		t.Fatal(err)
	}
	if c.Topology != "leaderless" || c.R != 1 || c.W != 5 || c.PeerCount != 1 {
		t.Fatalf("config %+v", c)
	}
}
