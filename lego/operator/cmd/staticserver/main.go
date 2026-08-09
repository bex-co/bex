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

// Command staticserver is the shared static-site origin proxy (w1/m21): the
// always-on serving half of the static_site service type. It watches static_site
// Apps for the host→revision mapping and their edge rules, and serves their
// built output from the object-store origin with signed GETs. The operator
// points each static-site host's Ingress at this server's Service (the same
// by-name backend wiring the wake activator uses). See docs/ADR029-static-sites.md.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/bex-co/bex/lego/operator/internal/hostingdomain"
	"github.com/bex-co/bex/lego/operator/internal/staticserver"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var setupLog = ctrl.Log.WithName("staticserver")

const (
	// Public-server timeouts bound slow-header/slow-read/idle connections so a
	// trickle client cannot pin a goroutine + fd open on the shared single-replica
	// server (finding 12). ReadHeaderTimeout caps slow request headers;
	// WriteTimeout caps the response phase (generous so a large-but-legitimate
	// asset over a slow link still completes); IdleTimeout caps keep-alive idle.
	readHeaderTimeout = 10 * time.Second
	writeTimeout      = 120 * time.Second
	idleTimeout       = 120 * time.Second
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// newServer builds the static-server HTTP server with the bounding timeouts set
// (finding 12). Extracted so the timeout wiring is unit-tested.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	addr := envOr("BEX_STATIC_ADDR", ":8080")
	namespace := os.Getenv("BEX_STATIC_NAMESPACE") // empty => all namespaces
	baseDomain := os.Getenv("BEX_BASE_DOMAIN")
	if err := hostingdomain.ValidateSharedSuffix(baseDomain); err != nil {
		// Mirror the manager: a well-formed-but-not-yet-PSL-listed suffix
		// (onbex.co) serves with a warning instead of crashing; a malformed
		// suffix stays fatal.
		if errors.Is(err, hostingdomain.ErrUnlistedSharedSuffix) {
			setupLog.Info("WARNING: BEX_BASE_DOMAIN is not a private Public Suffix in this build; serving shared platform hosts anyway. Cross-tenant cookie isolation is not browser-enforced until this suffix is in the PSL — submit onbex.co to publicsuffix/list",
				"baseDomain", baseDomain, "reason", err.Error())
		} else {
			setupLog.Error(err, "unsafe shared tenant hosting suffix")
			os.Exit(1)
		}
	}
	endpoint := os.Getenv("BEX_STATIC_S3_ENDPOINT")
	bucket := os.Getenv("BEX_STATIC_S3_BUCKET")
	region := os.Getenv("BEX_STATIC_S3_REGION")
	configured := endpoint != "" && bucket != ""

	resync := 10 * time.Second
	if v := os.Getenv("BEX_STATIC_RESYNC"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			resync = d
		}
	}
	var cacheBytes int64 = 256 << 20 // 256 MiB default
	if v := os.Getenv("BEX_STATIC_CACHE_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cacheBytes = n
		}
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "add client-go scheme")
		os.Exit(1)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "add app scheme")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	if !configured {
		// Degraded mode: stay Ready (so the platform can deploy the static-server
		// unconditionally) but answer content requests with 503 until the object
		// store is configured (BEX_STATIC_S3_ENDPOINT/BUCKET + the creds Secret).
		setupLog.Info("static origin not configured; serving 503 until BEX_STATIC_S3_ENDPOINT/BUCKET are set")
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "static origin not configured", http.StatusServiceUnavailable)
		})
	} else {
		cfg, err := ctrl.GetConfig()
		if err != nil {
			setupLog.Error(err, "load kube config")
			os.Exit(1)
		}
		cl, err := client.New(cfg, client.Options{Scheme: scheme})
		if err != nil {
			setupLog.Error(err, "build kube client")
			os.Exit(1)
		}
		origin, err := staticserver.NewS3Origin(ctx, endpoint, region, bucket)
		if err != nil {
			setupLog.Error(err, "build s3 origin")
			os.Exit(1)
		}
		if err := origin.Check(ctx); err != nil {
			setupLog.Error(err, "verify static origin credential")
			os.Exit(1)
		}
		resolver := staticserver.NewCachedResolver(cl, namespace, baseDomain)
		go resolver.Run(ctx, resync, func(err error) {
			setupLog.Error(err, "refresh static-site snapshot")
		})
		mux.Handle("/", staticserver.New(resolver, origin, cacheBytes))
	}

	srv := newServer(addr, mux)
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	setupLog.Info("static-server listening", "addr", addr, "bucket", bucket, "namespace", namespace)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		setupLog.Error(err, "serve")
		os.Exit(1)
	}
}
