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
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Holding a request through a cold start (w8/178). Render delays the first
// request to a spun-down free service until the service is back up and then
// serves it, so a webhook, bot or API client loses nothing; only a browser gets
// a loading page. The activator therefore wakes the workload as before, then —
// for a client that does not accept text/html — holds the request until a ready
// instance exists and reverse-proxies it to the App's Service. A request still
// unserved at holdBound gets a gateway timeout.

const (
	// holdBound is how long a request may wait for a wake: Render's spin-up
	// "takes about one minute" and "50 seconds or more".
	holdBound = 90 * time.Second
	// holdPollInterval is how often a wake's readiness is read.
	holdPollInterval = 500 * time.Millisecond
	// maxHeldRequests and maxHeldPerApp bound the goroutines, sockets and
	// memory a flood of requests to sleeping hosts can pin; past them the
	// request gets the immediate not-ready answer.
	maxHeldRequests = 256
	maxHeldPerApp   = 64
	// dialRetryInterval spaces the connection attempts while the Service's
	// endpoint propagates to kube-proxy after the pod turns ready.
	dialRetryInterval = 250 * time.Millisecond
)

// holder holds requests for sleeping Apps and proxies them once awake.
type holder struct {
	c            client.Client
	bound        time.Duration
	pollInterval time.Duration
	// target is where a woken App's Service listens; tests point it elsewhere.
	target func(app *appv1alpha1.App) *url.URL
	// ready decides whether the App can take traffic; tests replace it.
	ready func(ctx context.Context, app *appv1alpha1.App) bool

	total  chan struct{}
	mu     sync.Mutex
	perApp map[string]int
	gates  map[string]*readyGate
	waking map[string]time.Time
	proxy  *httputil.ReverseProxy
}

// readyGate is one App's wake in flight: every held request waits on it, and a
// single poller resolves it.
type readyGate struct {
	done chan struct{}
	ok   bool
}

func newHolder(c client.Client) *holder {
	h := &holder{
		c: c, bound: holdBound, pollInterval: holdPollInterval,
		total:  make(chan struct{}, maxHeldRequests),
		perApp: map[string]int{}, gates: map[string]*readyGate{}, waking: map[string]time.Time{},
		target: serviceURL,
	}
	h.ready = h.deploymentReady
	h.proxy = h.newProxy()
	return h
}

// serviceURL is the App's cluster Service, which the operator names after the App.
func serviceURL(app *appv1alpha1.App) *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(app.Name+"."+app.Namespace+".svc", strconv.Itoa(int(app.ServingPort()))),
	}
}

func (h *holder) deploymentReady(ctx context.Context, app *appv1alpha1.App) bool {
	dep := &appsv1.Deployment{}
	if err := h.c.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, dep); err != nil {
		return false
	}
	return dep.Status.ReadyReplicas >= 1
}

type deadlineKey struct{}

// forwardedHeaders are the forwarding headers the trusted ingress sets.
var forwardedHeaders = []string{
	"X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host", "X-Forwarded-Port", "X-Real-Ip",
}

// newProxy builds the reverse proxy to a woken App. The public host, and the
// forwarding headers the trusted ingress set, reach the App unchanged. A
// connection refused while the Service's endpoint propagates is retried until
// the request's hold deadline; nothing has been sent by then, so a request with
// a body is safe to retry.
func (h *holder) newProxy() *httputil.ReverseProxy {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			app, _ := pr.In.Context().Value(appKey{}).(*appv1alpha1.App)
			pr.SetURL(h.target(app))
			pr.Out.Host = pr.In.Host
			for _, name := range forwardedHeaders {
				if v := pr.In.Header.Values(name); len(v) > 0 {
					pr.Out.Header[name] = v
				}
			}
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				deadline, _ := ctx.Value(deadlineKey{}).(time.Time)
				for {
					conn, err := dialer.DialContext(ctx, network, addr)
					if err == nil {
						return conn, nil
					}
					if ctx.Err() != nil || deadline.IsZero() || !time.Now().Add(dialRetryInterval).Before(deadline) {
						return nil, err
					}
					select {
					case <-ctx.Done():
						return nil, err
					case <-time.After(dialRetryInterval):
					}
				}
			},
			MaxIdleConns:        64,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			writeBody(w, &http.Request{Method: http.MethodGet}, http.StatusBadGateway,
				"application/json", `{"error":"bad gateway"}`)
		},
	}
}

type appKey struct{}

// holding reports whether requests for key should be held: the App is hibernated
// or a wake this process started is still in flight.
func (h *holder) holding(app *appv1alpha1.App, key string) bool {
	if app.Status.Phase == appv1alpha1.PhaseHibernated {
		return true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	until, ok := h.waking[key]
	if ok && time.Now().After(until) {
		delete(h.waking, key)
		return false
	}
	return ok
}

// markWaking records a wake in flight until the gate resolves.
func (h *holder) markWaking(key string) {
	h.mu.Lock()
	h.waking[key] = time.Now().Add(h.bound + 30*time.Second)
	h.mu.Unlock()
}

// admit reserves a held-request slot; false means shed with the immediate answer.
func (h *holder) admit(key string) (release func(), ok bool) {
	select {
	case h.total <- struct{}{}:
	default:
		return nil, false
	}
	h.mu.Lock()
	if h.perApp[key] >= maxHeldPerApp {
		h.mu.Unlock()
		<-h.total
		return nil, false
	}
	h.perApp[key]++
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		h.perApp[key]--
		if h.perApp[key] <= 0 {
			delete(h.perApp, key)
		}
		h.mu.Unlock()
		<-h.total
	}, true
}

// wait blocks until the App is ready (true) or the hold bound or the request
// ends (false). One poller per App serves every waiter.
func (h *holder) wait(ctx context.Context, app *appv1alpha1.App, key string) bool {
	h.mu.Lock()
	g, running := h.gates[key]
	if !running {
		g = &readyGate{done: make(chan struct{})}
		h.gates[key] = g
		go h.poll(app.DeepCopy(), key, g)
	}
	h.mu.Unlock()
	select {
	case <-g.done:
		return g.ok
	case <-ctx.Done():
		return false
	}
}

func (h *holder) poll(app *appv1alpha1.App, key string, g *readyGate) {
	ctx, cancel := context.WithTimeout(context.Background(), h.bound)
	defer cancel()
	tick := time.NewTicker(h.pollInterval)
	defer tick.Stop()
	for {
		if h.ready(ctx, app) {
			g.ok = true
			break
		}
		select {
		case <-ctx.Done():
		case <-tick.C:
			continue
		}
		break
	}
	h.mu.Lock()
	delete(h.gates, key)
	delete(h.waking, key)
	h.mu.Unlock()
	close(g.done)
}

// serve holds r until app is ready and proxies it, answering a gateway timeout
// at the bound. It reports false, having written nothing, when the request was
// shed by the held-request limits, so the caller gives the immediate answer.
func (h *holder) serve(w http.ResponseWriter, r *http.Request, app *appv1alpha1.App, key string) bool {
	release, ok := h.admit(key)
	if !ok {
		return false
	}
	defer release()
	// The held request outlives the server's header/body/write deadlines.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	start := time.Now()
	waitCtx, cancel := context.WithTimeout(r.Context(), h.bound)
	defer cancel()
	if !h.wait(waitCtx, app, key) {
		if r.Context().Err() == nil {
			writeBody(w, r, http.StatusGatewayTimeout, "application/json",
				`{"error":"service did not wake in time","retryAfter":5}`)
		}
		return true
	}
	ctx := context.WithValue(r.Context(), appKey{}, app)
	ctx = context.WithValue(ctx, deadlineKey{}, start.Add(h.bound))
	h.proxy.ServeHTTP(w, r.WithContext(ctx))
	return true
}
