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

// Command listener-reader is the node agent that tells the operator which TCP
// ports a pod on its node listens on, read from the pod's own socket table
// (internal/listeners, w6/m530), so the operator never connects to every port
// of a pod to find out. It runs as a DaemonSet (config/listener-reader) with
// the node's PID namespace as its only privilege, and answers the operator
// alone (its NetworkPolicy).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/listeners"
)

func main() {
	// In its own PID namespace the reader is PID 1 and sees no pod at all, and
	// every read would answer "not on this node". Refuse to start instead.
	if os.Getpid() == 1 {
		log.Fatal("listener-reader: running as PID 1, so not in the node's PID namespace; set hostPID: true")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(listeners.Port),
		Handler:           listeners.Handler("/proc"),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       10 * time.Minute,
		MaxHeaderBytes:    8 << 10,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("listener-reader: serving %s on :%d", listeners.Path, listeners.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listener-reader: %v", err)
	}
}
