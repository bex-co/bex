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

package listeners

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

const (
	// Port is where the reader serves on its pod address
	// (config/listener-reader/daemonset.yaml's listeners port).
	Port = 9097
	// Path is the one read the reader answers.
	Path = "/v1/listeners"
	// maxConcurrentReads bounds the reads in flight; one walks the node's
	// /proc, so a burst queues instead of competing for the CPU limit.
	maxConcurrentReads = 4
	// readTimeout bounds one read, queueing included.
	readTimeout = 5 * time.Second
)

// Answer is the reader's reply to a read: the pod's listening ports.
type Answer struct {
	Ports []int `json:"ports"`
}

// Handler serves GET Path?pod=<uid>&ip=<address> for the pods of this node,
// read under procRoot, plus /healthz and /readyz. A read answers 200 with an
// Answer, 400 for a malformed request, 404 when the pod has no process on this
// node, 409 when its namespace does not hold that address, 503 when the reader
// is saturated or out of time, and 500 when the tables could not be read. It
// answers ports only: no path, process or address leaves the node.
func Handler(procRoot string) http.Handler {
	slots := make(chan struct{}, maxConcurrentReads)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+Path, func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength != 0 || len(r.TransferEncoding) > 0 {
			http.Error(w, "a read carries no body", http.StatusBadRequest)
			return
		}
		query := r.URL.Query()
		podIP, err := netip.ParseAddr(query.Get("ip"))
		if err != nil || len(query["pod"]) != 1 || len(query["ip"]) != 1 {
			http.Error(w, ErrBadRequest.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), readTimeout)
		defer cancel()
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		case <-ctx.Done():
			http.Error(w, "listener reader busy", http.StatusServiceUnavailable)
			return
		}
		ports, err := Read(ctx, procRoot, query.Get("pod"), podIP)
		if err != nil {
			status := statusOf(err)
			if status == http.StatusInternalServerError {
				log.Printf("listener-reader: pod %s: %v", query.Get("pod"), err)
			}
			http.Error(w, errorText(err), status)
			return
		}
		if ports == nil {
			ports = []int{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Answer{Ports: ports})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Ready once the node's process table is readable: PID 1's cgroup is the
	// first file every read walks past.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := os.ReadFile(filepath.Join(procRoot, "1", "cgroup")); err != nil {
			http.Error(w, "the node's process table is unreadable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, ErrPodNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAddressMismatch):
		return http.StatusConflict
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

// errorText is the reply body for err: the sentinel's text, never the path or
// the kernel's message that a wrapped error carries.
func errorText(err error) string {
	for _, sentinel := range []error{ErrBadRequest, ErrPodNotFound, ErrAddressMismatch, ErrUnreadable} {
		if errors.Is(err, sentinel) {
			return sentinel.Error()
		}
	}
	return "listener read failed"
}
