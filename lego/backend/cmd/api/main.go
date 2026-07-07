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

// Command api is the bex control-plane seed: a service exposing REST, GraphQL
// and MCP for the App lifecycle verbs (list / get / restart / suspend / resume /
// logs). It shares the operator image but runs as a separate Deployment
// (command: /api). Default mode serves HTTP; `api mcp-stdio` (or BEX_MCP_STDIO=1)
// serves the MCP adapter over stdio for a local agent. It is a pure apiserver
// client — it patches App CRs and reads pod logs; the operator does the rest.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/api"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))

	cfg := ctrl.GetConfigOrDie() // in-cluster, or KUBECONFIG for local dev
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("kube client: %v", err)
	}
	// Clientset just for the pod-log subresource (the one read controller-runtime's
	// client can't serve); Core.Logs uses it via NewPodLogSource.
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("kube clientset: %v", err)
	}

	core := &api.Core{
		Client:        cl,
		Namespace:     envOr("BEX_API_NAMESPACE", "default"),
		PodLogs:       api.NewPodLogSource(cs),
		PodLogsFollow: api.NewPodLogStream(cs), // live tail for GET /v1/logs/subscribe
		// Resource metrics (cpu/memory) via metrics-server; instance count needs no
		// source. Left nil if metrics-server is absent => those metrics report 503.
		ResourceMetrics: api.NewResourceMetricsSource(cs),
	}
	// Request metrics (http_requests/latency/bandwidth) need Traefik scraped by
	// Prometheus; wired only when BEX_PROM_URL is set, else those metrics 503.
	if prom := os.Getenv("BEX_PROM_URL"); prom != "" {
		core.RequestMetrics = api.NewPrometheusRequestSource(prom, nil)
	}
	// Auth (docs/auth.md): OAuth2 API keys introspected at Hydra's admin API,
	// Kratos sessions optional. Handler() fails fast without the Hydra URL.
	// nil key store (stdio mode without a Hydra URL) keeps the api-key verbs
	// answering ErrAPIKeysUnavailable instead of dialing nowhere.
	hydraAdminURL := os.Getenv("BEX_HYDRA_ADMIN_URL")
	if hydraAdminURL != "" {
		core.APIKeys = api.NewHydraAPIKeys(hydraAdminURL)
	}
	// Authorization (docs/auth.md): unset => authz disabled (every verb allowed,
	// the pre-m4 behavior); set => every Core verb checks OpenFGA, fail closed.
	// NOT wired in stdio mode: that transport's trust boundary is the subprocess
	// itself (no auth gate, so no identity — a wired checker would deny all).
	if fga := os.Getenv("BEX_OPENFGA_URL"); fga != "" && !mcpStdio() {
		core.Authz = api.NewOpenFGAChecker(fga, os.Getenv("BEX_OPENFGA_TOKEN"))
	}

	srv := &api.Server{
		Core:          core,
		CORSOrigin:    os.Getenv("BEX_API_CORS_ORIGIN"),
		HydraAdminURL: hydraAdminURL,
		KratosURL:     os.Getenv("BEX_KRATOS_URL"),
	}

	// stdio MCP mode: `api mcp-stdio` (or BEX_MCP_STDIO=1) serves only the MCP
	// adapter over stdin/stdout — how a local agent launches bex as a subprocess.
	if mcpStdio() {
		log.Printf("bex-api: serving MCP over stdio (namespace %s)", srv.Core.Namespace)
		if err := srv.RunStdio(ctrl.SetupSignalHandler()); err != nil {
			log.Fatalf("bex-api mcp stdio: %v", err)
		}
		return
	}

	handler, err := srv.Handler()
	if err != nil {
		log.Fatalf("bex-api: %v", err)
	}

	addr := envOr("BEX_API_ADDR", ":8090")
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	log.Printf("bex-api listening on %s (namespace %s)", addr, srv.Core.Namespace)
	log.Fatal(httpSrv.ListenAndServe())
}

// mcpStdio reports whether the binary should run as a stdio MCP server rather
// than the HTTP service: subcommand `mcp-stdio` or BEX_MCP_STDIO=1.
func mcpStdio() bool {
	if os.Getenv("BEX_MCP_STDIO") == "1" {
		return true
	}
	return len(os.Args) > 1 && os.Args[1] == "mcp-stdio"
}
