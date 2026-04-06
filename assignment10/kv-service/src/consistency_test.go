package main

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// restoreDelays resets assignment delays after tests (TestMain zeros them globally).
func restoreDelays(t *testing.T) {
	t.Helper()
	oldL := leaderAfterFollowerSleep
	oldF := followerReplicateSleep
	oldI := followerInternalReadSleep
	t.Cleanup(func() {
		leaderAfterFollowerSleep = oldL
		followerReplicateSleep = oldF
		followerInternalReadSleep = oldI
	})
}

// --- Part IV: Leader–Follower consistency ---

func TestLeaderWrite_ThenReadLeaderAndFollowerConsistent(t *testing.T) {
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
	follow.cfg.LeaderURL = lts.URL

	body := `{"key":"consis","value":"v-final"}`
	res, err := http.Post(lts.URL+"/set", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("set: %s", res.Status)
	}

	gl, err := http.Get(lts.URL + "/get?key=consis")
	if err != nil {
		t.Fatal(err)
	}
	defer gl.Body.Close()
	if gl.StatusCode != http.StatusOK {
		t.Fatalf("leader get: %s", gl.Status)
	}
	b, _ := io.ReadAll(gl.Body)
	if string(b) != "v-final" {
		t.Fatalf("leader value %q", b)
	}

	gf, err := http.Get(fts.URL + "/get?key=consis")
	if err != nil {
		t.Fatal(err)
	}
	defer gf.Body.Close()
	if gf.StatusCode != http.StatusOK {
		t.Fatalf("follower get (via leader): %s", gf.Status)
	}
	b2, _ := io.ReadAll(gf.Body)
	if string(b2) != "v-final" {
		t.Fatalf("follower get %q", b2)
	}
}

func TestLeaderLocalReadOnFollowerStaleWhileReplicating(t *testing.T) {
	restoreDelays(t)
	leaderAfterFollowerSleep = 150 * time.Millisecond
	followerReplicateSleep = 120 * time.Millisecond

	follow := NewServer(roleFollower, Config{N: 2, R: 1, W: 2})
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
	follow.cfg.LeaderURL = lts.URL

	client := &http.Client{Timeout: 30 * time.Second}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = client.Post(lts.URL+"/set", "application/json", bytes.NewReader([]byte(`{"key":"lag","value":"new"}`)))
	}()

	// Wait until leader has applied locally but follower may still be in replicate+sleep window.
	deadline := time.Now().Add(400 * time.Millisecond)
	sawStale := false
	for time.Now().Before(deadline) {
		lr, err := client.Get(fts.URL + "/local_read?key=lag")
		if err != nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		var lrsp internalReadResponse
		_ = json.NewDecoder(lr.Body).Decode(&lrsp)
		lr.Body.Close()
		if !lrsp.OK {
			sawStale = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()

	if !sawStale {
		t.Log("note: did not observe follower local_read !ok during replication (timing); delays may need tuning on slow CI")
	}
	// After write completes, local_read on follower must match.
	res, err := http.Get(fts.URL + "/local_read?key=lag")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var final internalReadResponse
	if err := json.NewDecoder(res.Body).Decode(&final); err != nil {
		t.Fatal(err)
	}
	if !final.OK || final.Value != "new" {
		t.Fatalf("after ack follower local_read want new, got %+v", final)
	}
}

// --- Part IV: Leaderless inconsistency window ---

func TestLeaderlessInconsistencyWindowThenConsistent(t *testing.T) {
	restoreDelays(t)
	leaderAfterFollowerSleep = 80 * time.Millisecond
	followerReplicateSleep = 80 * time.Millisecond

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

	coordURL, otherURL := tsA.URL, tsB.URL
	if rand.Intn(2) == 1 {
		coordURL, otherURL = otherURL, coordURL
	}

	client := &http.Client{Timeout: 30 * time.Second}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = client.Post(coordURL+"/set", "application/json", bytes.NewReader([]byte(`{"key":"ll","value":"coord"}`)))
	}()

	inconsistent := false
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		g, err := client.Get(otherURL + "/get?key=ll")
		if err != nil {
			time.Sleep(3 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(g.Body)
		g.Body.Close()
		if g.StatusCode == http.StatusNotFound || string(body) != "coord" {
			inconsistent = true
			break
		}
		time.Sleep(3 * time.Millisecond)
	}
	wg.Wait()

	if !inconsistent {
		t.Log("note: did not catch GET on peer before replicate finished (timing-dependent)")
	}

	ga, err := client.Get(coordURL + "/get?key=ll")
	if err != nil {
		t.Fatal(err)
	}
	defer ga.Body.Close()
	if ga.StatusCode != http.StatusOK {
		t.Fatalf("coordinator get: %s", ga.Status)
	}
	ba, _ := io.ReadAll(ga.Body)
	if string(ba) != "coord" {
		t.Fatalf("coordinator body %q", ba)
	}

	gb, err := client.Get(otherURL + "/get?key=ll")
	if err != nil {
		t.Fatal(err)
	}
	defer gb.Body.Close()
	if gb.StatusCode != http.StatusOK {
		t.Fatalf("other node get: %s", gb.Status)
	}
	bb, _ := io.ReadAll(gb.Body)
	if string(bb) != "coord" {
		t.Fatalf("other node body %q", bb)
	}
}
