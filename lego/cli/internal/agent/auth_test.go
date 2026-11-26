/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/config"
)

// clock is the time a test's login and token server share.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// tokenServer is bex-api's token endpoint and an API behind it, as Hydra
// behaves: a refresh revokes every access token issued before it and spends
// the refresh token it traded. Any request it sees with a dead bearer is
// recorded, so a test can say none was sent.
type tokenServer struct {
	t     *testing.T
	srv   *httptest.Server
	clock *clock
	life  time.Duration

	mu        sync.Mutex
	serial    int
	live      map[string]time.Time // access token → expiry
	refresh   map[string]bool      // refresh token → not yet spent
	refreshes int
	dead      []string // "METHOD path" of requests that carried a dead bearer
	bearers   []string // the bearer of every API request, in order
	// onRefresh, when set, runs inside a refresh before it answers; refuse
	// makes the refresh answer invalid_grant.
	onRefresh func()
	refuse    bool
}

func newTokenServer(t *testing.T, routes map[string]http.HandlerFunc) *tokenServer {
	s := &tokenServer{t: t, clock: &clock{t: time.Unix(1_800_000_000, 0)}, life: 2 * time.Second,
		live: map[string]time.Time{}, refresh: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/token/refresh/", s.refreshToken)
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, s.authorized(handler))
	}
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

// mint issues a token pair, revoking every access token before it.
func (s *tokenServer) mint() (access, refresh string) {
	s.serial++
	access, refresh = fmt.Sprintf("at-%d", s.serial), fmt.Sprintf("rt-%d", s.serial)
	s.live = map[string]time.Time{access: s.clock.now().Add(s.life)}
	s.refresh[refresh] = true
	return access, refresh
}

// with runs f holding the server's lock, for a test changing its behavior.
func (s *tokenServer) with(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f()
}

// login is a fresh `bex login`: a token pair written to the login file.
func (s *tokenServer) login() config.APIConfig {
	s.mu.Lock()
	access, refresh := s.mint()
	c := config.APIConfig{Key: access, RefreshToken: refresh, Host: s.srv.URL + "/v1/",
		ExpiresAt: s.clock.now().Add(s.life).Unix()}
	s.mu.Unlock()
	if err := config.SetAPIConfig(c); err != nil {
		s.t.Fatal(err)
	}
	return c
}

// otherProcess is another `bex` command refreshing the login on file, as
// upstream does on every command in a token's last day.
func (s *tokenServer) otherProcess() config.APIConfig {
	s.t.Helper()
	disk, err := config.OAuthConfig()
	if err != nil {
		s.t.Fatal(err)
	}
	l := &login{cur: disk, now: s.clock.now, renewBefore: renewBefore}
	if err := l.renew(context.Background(), disk, true); err != nil {
		s.t.Fatal(err)
	}
	return l.cur
}

func (s *tokenServer) refreshToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		GrantType    string `json:"grant_type"`
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	s.mu.Lock()
	hook, refuse := s.onRefresh, s.refuse
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshes++
	if refuse || in.GrantType != "refresh_token" || !s.refresh[in.RefreshToken] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}
	s.refresh[in.RefreshToken] = false
	access, refresh := s.mint()
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "refresh_token": refresh,
		"token_type": "bearer", "expires_in": int(s.life / time.Second)})
}

func (s *tokenServer) authorized(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		s.mu.Lock()
		s.bearers = append(s.bearers, bearer)
		expiry, ok := s.live[bearer]
		alive := ok && s.clock.now().Before(expiry)
		if !alive {
			s.dead = append(s.dead, r.Method+" "+r.URL.Path)
		}
		s.mu.Unlock()
		if !alive {
			writeAPIError(w, http.StatusUnauthorized, "", "unauthorized")
			return
		}
		next(w, r)
	}
}

// revoke kills every access token, as signing out elsewhere does, and leaves
// the refresh tokens alone.
func (s *tokenServer) revoke() {
	s.mu.Lock()
	s.live = map[string]time.Time{}
	s.mu.Unlock()
}

func (s *tokenServer) counts() (refreshes int, dead, bearers []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshes, append([]string(nil), s.dead...), append([]string(nil), s.bearers...)
}

// api is a client of s holding the login on file, as defaultAPI builds one.
func (s *tokenServer) api(t *testing.T) *api {
	t.Helper()
	cur, err := config.DefaultAPIConfig()
	if err != nil {
		t.Fatal(err)
	}
	l := newLogin(cur)
	l.now, l.renewBefore = s.clock.now, time.Second
	return &api{server: s.srv.URL + "/v1/", http: s.srv.Client(), login: l,
		sleep: func(context.Context, time.Duration) error { return nil }}
}

// useLoginFile points the upstream config package at a login file of the
// test's own, with no API key in the environment.
func useLoginFile(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "cli.yaml")
	t.Setenv("RENDER_CLI_CONFIG_PATH", path)
	t.Setenv("RENDER_API_KEY", "")
	return path
}

func onFile(t *testing.T) config.APIConfig {
	t.Helper()
	disk, err := config.OAuthConfig()
	if err != nil {
		t.Fatal(err)
	}
	return disk
}

func sessionRoute(state string) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), state))
		},
	}
}

func TestALongRunRefreshesBeforeTheTokenExpires(t *testing.T) {
	useLoginFile(t)
	s := newTokenServer(t, sessionRoute(stateWorking))
	s.login()
	a := s.api(t)
	// Ten seconds of polling on two-second tokens.
	for range 20 {
		if _, _, err := a.session(context.Background(), "ags-long"); err != nil {
			t.Fatal(err)
		}
		s.clock.advance(500 * time.Millisecond)
	}
	refreshes, dead, _ := s.counts()
	if len(dead) > 0 {
		t.Fatalf("sent a dead bearer: %v", dead)
	}
	if refreshes < 4 {
		t.Fatalf("%d refreshes over ten seconds of two-second tokens", refreshes)
	}
	if disk := onFile(t); disk.Key != a.login.cur.Key || disk.RefreshToken != a.login.cur.RefreshToken {
		t.Fatalf("the login file holds %s, the run %s: a refresh was not written back", disk.Key, a.login.cur.Key)
	}
}

func TestALoginAnotherProcessRefreshedIsAdoptedWithoutARefresh(t *testing.T) {
	useLoginFile(t)
	s := newTokenServer(t, sessionRoute(stateWorking))
	s.with(func() { s.life = time.Hour }) // nothing here is near expiry
	s.login()
	a := s.api(t)
	if _, _, err := a.session(context.Background(), "ags-a"); err != nil {
		t.Fatal(err)
	}
	other := s.otherProcess() // revokes the token the run holds
	for range 3 {
		if _, _, err := a.session(context.Background(), "ags-a"); err != nil {
			t.Fatal(err)
		}
	}
	refreshes, dead, bearers := s.counts()
	if len(dead) > 0 || refreshes != 1 {
		t.Fatalf("%d refreshes (want the other process's one) and dead bearers %v", refreshes, dead)
	}
	if a.login.cur.Key != other.Key || bearers[len(bearers)-1] != other.Key {
		t.Fatalf("the run sends %s, not the login the other process wrote (%s)", a.login.cur.Key, other.Key)
	}
}

func TestARefreshTradesTheRefreshTokenOnFile(t *testing.T) {
	useLoginFile(t)
	s := newTokenServer(t, sessionRoute(stateWorking))
	s.login()
	a := s.api(t)
	if _, _, err := a.session(context.Background(), "ags-o"); err != nil {
		t.Fatal(err)
	}
	other := s.otherProcess() // spends the refresh token the run holds
	// Both logins near expiry: nothing to adopt, so the run refreshes, and
	// only the refresh token the other process left on file still trades.
	s.clock.advance(1500 * time.Millisecond)
	if _, _, err := a.session(context.Background(), "ags-o"); err != nil {
		t.Fatal(err)
	}
	refreshes, dead, _ := s.counts()
	if refreshes != 2 || len(dead) > 0 {
		t.Fatalf("%d refreshes, dead bearers %v: want the other's and one from %s", refreshes, dead, other.RefreshToken)
	}
	if disk := onFile(t); disk.Key != a.login.cur.Key {
		t.Fatalf("the login file holds %s, the run %s", disk.Key, a.login.cur.Key)
	}
}

func TestA401RefreshesOnceAndRepeatsOnlyARepeatableRequest(t *testing.T) {
	useLoginFile(t)
	var steers atomic.Int32
	routes := sessionRoute(stateWorking)
	routes["POST /v1/agent-sessions/{id}/steer"] = func(w http.ResponseWriter, r *http.Request) {
		steers.Add(1)
		writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), stateWorking))
	}
	s := newTokenServer(t, routes)
	s.with(func() { s.life = time.Hour })
	s.login()
	a := s.api(t)

	s.revoke() // the token dies with no newer login on file
	if _, _, err := a.session(context.Background(), "ags-r"); err != nil {
		t.Fatalf("a GET after a 401: %v", err)
	}
	refreshes, dead, bearers := s.counts()
	if refreshes != 1 || len(dead) != 1 || len(bearers) != 2 {
		t.Fatalf("%d refreshes, %d dead bearers, %d requests: want one refresh and one repeat", refreshes, len(dead), len(bearers))
	}

	s.revoke()
	_, _, err := a.steer(context.Background(), "ags-r", steerRequest{Prompt: "go on"}, "key-1")
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized || steers.Load() != 0 {
		t.Fatalf("a steer after a 401: err %v, %d steers delivered: want the 401 and no repeat", err, steers.Load())
	}
	if refreshes, _, _ := s.counts(); refreshes != 2 {
		t.Fatalf("%d refreshes: the steer's 401 renews the login for the next request", refreshes)
	}
	if _, _, err := a.steer(context.Background(), "ags-r", steerRequest{Prompt: "go on"}, "key-2"); err != nil || steers.Load() != 1 {
		t.Fatalf("the next steer: %v, %d delivered", err, steers.Load())
	}
}

func TestAFailedRefreshKeepsTheLoginAnotherProcessWrote(t *testing.T) {
	useLoginFile(t)
	s := newTokenServer(t, sessionRoute(stateWorking))
	s.login()
	a := s.api(t)
	var newer config.APIConfig
	s.with(func() {
		s.refuse = true
		s.onRefresh = func() {
			// `bex login` in another terminal while this refresh is in flight.
			s.with(func() { s.onRefresh, s.life = nil, time.Hour })
			newer = s.login()
		}
	})
	s.clock.advance(3 * time.Second) // the run's token has expired

	_, _, err := a.session(context.Background(), "ags-f")
	if err == nil || !strings.Contains(err.Error(), "invalid_grant; run `bex login`") {
		t.Fatalf("a refused refresh answered %v, want the login error", err)
	}
	if disk := onFile(t); disk.Key != newer.Key || disk.RefreshToken != newer.RefreshToken {
		t.Fatalf("the login file holds %q/%q, want the newer login %q/%q", disk.Key, disk.RefreshToken, newer.Key, newer.RefreshToken)
	}
	if _, _, err := a.session(context.Background(), "ags-f"); err != nil || a.login.cur.Key != newer.Key {
		t.Fatalf("after the other terminal's login: %v, sending %s", err, a.login.cur.Key)
	}
}

func TestAFailedRefreshKeepsAWorkingTokenAndPauses(t *testing.T) {
	useLoginFile(t)
	s := newTokenServer(t, sessionRoute(stateWorking))
	s.with(func() { s.life = time.Minute })
	s.login()
	a := s.api(t)
	a.login.renewBefore = 50 * time.Second
	s.with(func() { s.refuse = true })
	s.clock.advance(20 * time.Second) // inside renewBefore, still valid
	for range 3 {
		if _, _, err := a.session(context.Background(), "ags-p"); err != nil {
			t.Fatalf("the token still works: %v", err)
		}
		s.clock.advance(5 * time.Second)
	}
	if refreshes, dead, _ := s.counts(); refreshes != 1 || len(dead) > 0 {
		t.Fatalf("%d refreshes in 15 s, dead bearers %v: want one, then a pause", refreshes, dead)
	}
	s.clock.advance(refreshPause)
	if _, _, err := a.session(context.Background(), "ags-p"); err == nil {
		t.Fatal("a request after the token expired, with the refresh still refused, succeeded")
	}
	if refreshes, _, _ := s.counts(); refreshes != 2 {
		t.Fatalf("%d refreshes: want a second attempt after the pause", refreshes)
	}
}

func TestARefreshIsNotWrittenOverALoginWrittenMeanwhile(t *testing.T) {
	useLoginFile(t)
	s := newTokenServer(t, sessionRoute(stateWorking))
	s.login()
	a := s.api(t)
	var newer config.APIConfig
	s.with(func() {
		s.onRefresh = func() {
			s.with(func() { s.onRefresh = nil })
			newer = s.login()
		}
	})
	s.clock.advance(1500 * time.Millisecond) // inside renewBefore
	if _, _, err := a.session(context.Background(), "ags-w"); err != nil {
		t.Fatal(err)
	}
	if disk := onFile(t); disk.Key != newer.Key {
		t.Fatalf("the login file holds %s, want the login written during the refresh (%s)", disk.Key, newer.Key)
	}
}

func TestAnAPIKeyIsNeverRefreshed(t *testing.T) {
	useLoginFile(t)
	s := newTokenServer(t, sessionRoute(stateWorking))
	t.Setenv("RENDER_API_KEY", "bex_key")
	a := s.api(t)
	_, _, err := a.session(context.Background(), "ags-k")
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("a refused API key answered %v, want its 401", err)
	}
	if refreshes, _, bearers := s.counts(); refreshes != 0 || len(bearers) != 1 || bearers[0] != "bex_key" {
		t.Fatalf("%d refreshes, bearers %v", refreshes, bearers)
	}
}

func TestAStreamOutlivesItsTokenAndReconnectsWithAFreshOne(t *testing.T) {
	useLoginFile(t)
	var connects int
	var mu sync.Mutex
	var s *tokenServer
	fixture := replayFixture(t)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("the stream was sent a bearer")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		mu.Lock()
		connects++
		first := connects == 1
		mu.Unlock()
		if first {
			// The connection outlives the token, then drops mid-turn.
			_, _ = w.Write([]byte(fixture[:strings.Index(fixture, "data: {\"type\":\"text-start\"")]))
			s.clock.advance(5 * time.Second)
			return
		}
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(gateway.Close)
	s = newTokenServer(t, map[string]http.HandlerFunc{
		"POST /v1/agent-sessions/{id}/attach-ticket": func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"ticket": "tkt", "streamUrl": gateway.URL})
		},
		"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			state := stateWorking
			if connects >= 2 {
				state = stateSucceeded
			}
			writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), state))
		},
	})
	s.login()
	a := s.api(t)
	stdout, stderr, code := run(t, stubDeps(func() (*api, error) { return a, nil }), "-o", "text", "logs", "ags-live", "--follow")
	if code != 0 || stdout != renderedReplay {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
	refreshes, dead, _ := s.counts()
	mu.Lock()
	defer mu.Unlock()
	if len(dead) > 0 || refreshes == 0 || connects != 2 {
		t.Fatalf("%d refreshes, %d connections, dead bearers %v", refreshes, connects, dead)
	}
}
