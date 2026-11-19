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

package sshgateway

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
)

// ErrGatewayShutdown is the cause a terminating gateway ends the sessions
// still open when its drain bound passes (w6/m482, docs/ADR035-ssh.md
// § Sessions across a gateway release).
var ErrGatewayShutdown = errors.New("the ssh gateway shut down")

// Inflight counts one listener's open sessions, so a terminating gateway can
// wait for them once it has stopped accepting (w6/m482). The zero value is
// ready to use; Name labels its drain logs.
type Inflight struct {
	Name string

	mu       sync.Mutex
	open     int
	idle     chan struct{} // closed when open drops back to zero
	draining bool
}

// Begin counts one session in. Every Begin needs one End.
func (f *Inflight) Begin() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.open == 0 {
		f.idle = make(chan struct{})
	}
	f.open++
}

// End counts one session out; once the drain has begun, it logs each end.
func (f *Inflight) End() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.open--
	if f.draining {
		log.Printf("ssh gateway: draining %s: a session ended, %d still open", f.Name, f.open)
	}
	if f.open == 0 {
		close(f.idle)
	}
}

// Wait returns once no session is open, or returns ctx's error once ctx is
// done. The first call starts the drain: it logs how many sessions are open.
func (f *Inflight) Wait(ctx context.Context) error {
	f.mu.Lock()
	if !f.draining {
		f.draining = true
		log.Printf("ssh gateway: draining %s: %d sessions open", f.Name, f.open)
	}
	if f.open == 0 {
		f.mu.Unlock()
		return nil
	}
	idle := f.idle
	f.mu.Unlock()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Handler counts every request h serves as a session, including one whose
// connection h hijacks (a WebSocket, a CONNECT tunnel), which
// http.Server.Shutdown neither tracks nor waits for.
func (f *Inflight) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.Begin()
		defer f.End()
		h.ServeHTTP(w, r)
	})
}
