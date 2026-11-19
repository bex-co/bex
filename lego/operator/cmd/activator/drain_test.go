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

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m485: a terminating activator keeps serving through its drain window,
// then answers every request it still holds before it exits, at the latest at
// its bound.

// drainHelperEnv makes the test binary run as the activator's listener
// (listenAndDrain), so the SIGTERM test signals a real process.
const drainHelperEnv = "BEX_ACTIVATOR_DRAIN_UNDER_TEST"

const (
	helperWindow  = 500 * time.Millisecond
	helperHoldFor = 1500 * time.Millisecond
)

func TestMain(m *testing.M) {
	if os.Getenv(drainHelperEnv) == "1" {
		os.Exit(drainHelper())
	}
	os.Exit(m.Run())
}

// drainHelper serves /hold, which answers after helperHoldFor and says so on
// stdout when it starts, and any other path at once.
func drainHelper() int {
	out := funcr.New(func(_, args string) { fmt.Println(args) }, funcr.Options{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hold" {
			fmt.Println("holding")
			time.Sleep(helperHoldFor)
		}
		_, _ = io.WriteString(w, "answered "+r.URL.Path)
	})
	if err := listenAndDrain(out, "127.0.0.1:0", handler, helperWindow); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}

// get fetches url on a fresh connection, as a client the endpoint withdrawal
// has not reached yet does.
func get(target string) (int, string, error) {
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 10 * time.Second}
	resp, err := client.Get(target)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), err
}

// The process catches SIGTERM: the request it holds at the signal is answered,
// a request in the drain window is served, and it exits 0 once it is idle.
func TestSIGTERMDrainsTheActivatorProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), drainHelperEnv+"=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	lines := make(chan string, 16)
	exited := make(chan error, 1)
	go func() {
		for scan := bufio.NewScanner(stdout); scan.Scan(); {
			lines <- scan.Text()
		}
		close(lines)
		exited <- cmd.Wait()
	}()
	next := func(want string) string {
		t.Helper()
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("the activator exited before printing %q", want)
				}
				if strings.Contains(line, want) {
					return line
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("the activator never printed %q", want)
			}
		}
	}
	_, addr, _ := strings.Cut(next(`"msg"="listening"`), `"addr"="`)
	base := "http://" + strings.TrimSuffix(addr, `"`)

	type answer struct {
		code int
		body string
		err  error
		at   time.Time
	}
	held := make(chan answer, 1)
	go func() {
		code, body, err := get(base + "/hold")
		held <- answer{code, body, err, time.Now()}
	}()
	next("holding")
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	signaled := time.Now()

	time.Sleep(helperWindow / 5)
	if code, body, err := get(base + "/late"); err != nil || code != http.StatusOK || body != "answered /late" {
		t.Errorf("a request inside the drain window got %d %q, %v; want it served", code, body, err)
	}
	got := <-held
	if got.err != nil || got.code != http.StatusOK || got.body != "answered /hold" {
		t.Fatalf("the request held at SIGTERM got %d %q, %v; want its answer", got.code, got.body, got.err)
	}
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("the activator exited with %v after its drain, want 0", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the activator did not exit once idle")
	}
	if exited := time.Since(got.at); exited > 3*time.Second {
		t.Errorf("the activator exited %v after its last answer, want as soon as it was idle", exited)
	}
	if took := time.Since(signaled); took < helperWindow {
		t.Errorf("the activator exited %v after SIGTERM, inside its %v window", took, helperWindow)
	}
}

// drainServer serves handler through serveUntilDrained on a loopback port. It
// returns the server's address, the signal, and the drain's result.
func drainServer(t *testing.T, handler http.Handler, window, bound time.Duration) (string, func(), <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop, signal := context.WithCancel(context.Background())
	t.Cleanup(signal)
	drained := make(chan error, 1)
	go func() {
		drained <- serveUntilDrained(stop, &http.Server{Handler: handler}, ln, window, bound)
	}()
	return ln.Addr().String(), signal, drained
}

// refused reports whether a new connection to addr is refused within wait.
func refused(addr string, wait time.Duration) bool {
	for deadline := time.Now().Add(wait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			return true
		}
		_ = conn.Close()
	}
	return false
}

// A request held for a sleeping App when the signal arrives is still answered
// by the App, even once the activator has stopped accepting.
func TestRequestHeldAtTheSignalGetsTheAppsAnswer(t *testing.T) {
	app, dep := sleepingApp()
	app.Status.Phase = appv1alpha1.PhaseHibernated
	cache, base := primedHostCache(t, app, dep)
	cl := &countingClient{Client: base}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(append([]byte("app:"), body...))
	}))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL)
	hold := newHolder(cl)
	hold.pollInterval = 10 * time.Millisecond
	hold.target = func(*appv1alpha1.App) *url.URL { return target }
	var awake atomic.Bool
	holding := make(chan struct{}, 1)
	hold.ready = func(context.Context, *appv1alpha1.App) bool {
		select {
		case holding <- struct{}{}:
		default:
		}
		return awake.Load()
	}
	handler := newHoldingHandler(cl, cache, logr.Discard(), hold)
	addr, signal, drained := drainServer(t, handler, 50*time.Millisecond, 10*time.Second)

	type answer struct {
		code int
		body string
		err  error
	}
	answered := make(chan answer, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/hooks/x", strings.NewReader("payload"))
		req.Host = "sleeping.onbex.co"
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			answered <- answer{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		answered <- answer{resp.StatusCode, string(body), err}
	}()
	<-holding
	signal()
	if !refused(addr, 2*time.Second) {
		t.Fatal("the activator still accepts connections after its window")
	}
	select {
	case err := <-drained:
		t.Fatalf("the drain ended (%v) while a request was still held", err)
	case <-time.After(100 * time.Millisecond):
	}
	awake.Store(true)
	got := <-answered
	if got.err != nil || got.code != http.StatusAccepted || got.body != "app:payload" {
		t.Fatalf("the held request got %d %q, %v; want the App's own answer", got.code, got.body, got.err)
	}
	if err := <-drained; err != nil {
		t.Fatalf("drain = %v, want a clean drain", err)
	}
}

// The window keeps serving new connections, the listener closes after it, and
// an idle activator ends its drain right then.
func TestTheDrainWindowServesThenTheDrainEndsOnceIdle(t *testing.T) {
	const window = 300 * time.Millisecond
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	addr, signal, drained := drainServer(t, ok, window, 10*time.Second)
	if code, _, err := get("http://" + addr + "/"); err != nil || code != http.StatusOK {
		t.Fatalf("before the signal: %d, %v", code, err)
	}
	signal()
	signaled := time.Now()
	time.Sleep(window / 3)
	if code, body, err := get("http://" + addr + "/"); err != nil || code != http.StatusOK || body != "ok" {
		t.Fatalf("inside the window: %d %q, %v; want it served", code, body, err)
	}
	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("drain = %v, want a clean drain", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("an idle drain did not end")
	}
	if took := time.Since(signaled); took < window || took > window+2*time.Second {
		t.Fatalf("an idle drain ended %v after the signal, want right after its %v window", took, window)
	}
	if !refused(addr, time.Second) {
		t.Fatal("the listener still accepts after the drain")
	}
}

// A request still open at the bound does not hold the process past it.
func TestTheDrainIsBounded(t *testing.T) {
	const bound = 200 * time.Millisecond
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	started := make(chan struct{})
	stuck := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})
	addr, signal, drained := drainServer(t, stuck, 0, bound)
	go func() { _, _, _ = get("http://" + addr + "/") }()
	<-started
	signal()
	signaled := time.Now()
	select {
	case err := <-drained:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("drain = %v, want the bound's deadline", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the drain outlived its bound")
	}
	if took := time.Since(signaled); took < bound {
		t.Fatalf("the drain gave up after %v, before its %v bound", took, bound)
	}
}

// http.Server.Shutdown forgets a hijacked connection, so the drain waits for
// the handler that serves it: an upgraded stream shorter than the bound ends
// on its own terms.
func TestTheDrainWaitsForAHijackedStream(t *testing.T) {
	const streamFor = 300 * time.Millisecond
	upgraded := make(chan struct{})
	stream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: test\r\n\r\n")
		_ = buf.Flush()
		close(upgraded)
		time.Sleep(streamFor)
		_, _ = buf.WriteString("done")
		_ = buf.Flush()
	})
	addr, signal, drained := drainServer(t, stream, 0, 10*time.Second)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_, _ = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: test\r\n\r\n")
	<-upgraded
	signal()
	signaled := time.Now()
	select {
	case err := <-drained:
		if err != nil {
			t.Fatalf("drain = %v, want a clean drain", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the drain did not end after the stream")
	}
	if took := time.Since(signaled); took < streamFor*2/3 {
		t.Fatalf("the drain ended %v after the signal, while the %v stream was still open", took, streamFor)
	}
	rest, _ := io.ReadAll(conn)
	if !strings.HasSuffix(string(rest), "done") {
		t.Fatalf("the stream read %q, want it to end with its last bytes", rest)
	}
}

func TestDrainWindowFromEnv(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{"", defaultDrainWindow, true},
		{"0", 0, true},
		{"15", 15 * time.Second, true},
		{"-1", 0, false},
		{"1.5", 0, false},
		{"5s", 0, false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("BEX_ACTIVATOR_DRAIN_SECONDS", tc.value)
			got, err := drainWindowFromEnv()
			if (err == nil) != tc.ok || got != tc.want {
				t.Fatalf("drainWindowFromEnv() = %v, %v; want %v, ok=%v", got, err, tc.want, tc.ok)
			}
		})
	}
}

// The shipped Deployment lets a terminating pod finish its window and its
// bound before the kubelet kills it, and brings the replacement up first.
func TestTheDeploymentGivesTheDrainItsBound(t *testing.T) {
	raw, err := os.ReadFile("../../config/activator/deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var dep appsv1.Deployment
	if err := yaml.Unmarshal(raw, &dep); err != nil {
		t.Fatal(err)
	}
	window := defaultDrainWindow
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "BEX_ACTIVATOR_DRAIN_SECONDS" {
			t.Setenv(env.Name, env.Value)
			if window, err = drainWindowFromEnv(); err != nil {
				t.Fatal(err)
			}
		}
	}
	need := int64(math.Ceil((window + drainBound).Seconds()))
	if grace := dep.Spec.Template.Spec.TerminationGracePeriodSeconds; grace == nil || *grace <= need {
		t.Fatalf("terminationGracePeriodSeconds = %v, want more than the %ds drain window plus bound", grace, need)
	}
	rolling := dep.Spec.Strategy.RollingUpdate
	if dep.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType || rolling == nil ||
		rolling.MaxUnavailable == nil || rolling.MaxUnavailable.IntValue() != 0 ||
		rolling.MaxSurge == nil || rolling.MaxSurge.IntValue() < 1 {
		t.Fatalf("strategy = %+v, want a RollingUpdate that surges before an old pod drains", dep.Spec.Strategy)
	}
}
