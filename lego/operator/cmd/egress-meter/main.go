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
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/bex-co/bex/lego/operator/internal/egressmeter"
)

const defaultMetricsAddr = ":9091"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("egress-meter: kubernetes config: %v", err)
	}
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatalf("egress-meter: kubernetes client: %v", err)
	}
	meter := egressmeter.New(egressmeter.Config{
		NodeName:      os.Getenv("BEX_EGRESS_NODE_NAME"),
		Namespace:     os.Getenv("BEX_APPS_NAMESPACE"),
		ExcludedCIDRs: csv(os.Getenv("BEX_EGRESS_EXCLUDED_CIDRS")),
	}, client)

	registry := prometheus.NewRegistry()
	registry.MustRegister(egressmeter.NewCollector(meter))
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !meter.Ready() {
			http.Error(w, "accounting source incomplete", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	addr := os.Getenv("BEX_EGRESS_METRICS_ADDR")
	if addr == "" {
		addr = defaultMetricsAddr
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("egress-meter: metrics server: %v", err)
			stop()
		}
	}()
	go meter.Run(ctx)
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func csv(raw string) []string {
	var values []string
	for value := range strings.SplitSeq(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}
