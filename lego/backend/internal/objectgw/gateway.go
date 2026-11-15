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

package objectgw

import (
	"io"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// The gateway's two listeners (ADR112 D4). The public one is what Traefik routes
// pub-<xid> hosts to; the internal one carries the probe and the metrics and is
// never routed to.
const (
	PublicAddr   = ":8080"
	InternalAddr = ":9090"
)

// Public-server timeouts bound a slow reader so a trickle of connections cannot
// pin the replica (the static-server's reasoning, w4/029). A GET has no body to
// read, so ReadTimeout is short; WriteTimeout is the default a response gets,
// which each GET lifts to StreamDeadline.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 120 * time.Second
	idleTimeout       = 120 * time.Second
)

// Credentials are the gateway's whole environment (ADR112 D9): a read-only
// Wasabi identity, and the bearer it speaks to bex-api with.
type Credentials struct {
	ReadAccessKey string // BEX_OBJECT_STORAGE_READ_ACCESS_KEY
	ReadSecretKey string // BEX_OBJECT_STORAGE_READ_SECRET_KEY
	Token         string // BEX_OBJECT_STORAGE_GATEWAY_TOKEN
}

// Missing names the variables that are not set. A gateway with any of them
// missing serves nothing (Degraded), and says which.
func (c Credentials) Missing() []string {
	var missing []string
	for _, v := range []struct{ name, value string }{
		{"BEX_OBJECT_STORAGE_READ_ACCESS_KEY", c.ReadAccessKey},
		{"BEX_OBJECT_STORAGE_READ_SECRET_KEY", c.ReadSecretKey},
		{"BEX_OBJECT_STORAGE_GATEWAY_TOKEN", c.Token},
	} {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	return missing
}

// Config builds a Gateway. Production sets only Credentials; the rest default to
// the constants and tests replace them.
type Config struct {
	Credentials Credentials
	// BexAPIURL defaults to BexAPIURL.
	BexAPIURL string
	// Regions defaults to every region's production Wasabi connection with the
	// read-only credential.
	Regions map[objectstorage.Region]objectstorage.Config
	// Limits defaults to DefaultLimits.
	Limits *LimitConfig
	// HTTPClient carries the calls to bex-api; nil uses a client with a timeout.
	HTTPClient *http.Client
}

// Gateway is the assembled server: the public handler, the internal one, and
// the meter whose Run the process starts (nil in a degraded gateway).
type Gateway struct {
	Public   http.Handler
	Internal http.Handler
	Meter    *Meter
}

// New assembles the gateway from its credentials.
func New(cfg Config) *Gateway {
	apiURL := cfg.BexAPIURL
	if apiURL == "" {
		apiURL = BexAPIURL
	}
	regions := cfg.Regions
	if regions == nil {
		regions = map[objectstorage.Region]objectstorage.Config{}
		for _, r := range objectstorage.Regions {
			regions[r] = r.Config(cfg.Credentials.ReadAccessKey, cfg.Credentials.ReadSecretKey)
		}
	}
	limits := DefaultLimits()
	if cfg.Limits != nil {
		limits = *cfg.Limits
	}
	metrics := NewMetrics()
	api := APIClient{URL: apiURL, Token: cfg.Credentials.Token, Client: cfg.HTTPClient}
	meter := NewMeter(&HTTPReporter{APIClient: api})
	handler := NewHandler(
		NewCachedResolver(&HTTPLookup{APIClient: api}),
		NewWasabiOrigin(regions), NewLimits(limits), meter, metrics,
	)
	return &Gateway{Public: handler, Internal: internalMux(metrics), Meter: meter}
}

// NewDegraded is the gateway without its credentials: it stays up and Ready so a
// cluster without Wasabi (a local one) does not crash-loop, and answers every
// request 503. Public bucket hosting is then simply not available. It has no
// Meter: there is nothing to report.
func NewDegraded() *Gateway {
	degraded := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Retry-After", "60")
		writePlain(w, http.StatusServiceUnavailable, "public buckets are not available")
	})
	return &Gateway{Public: degraded, Internal: internalMux(NewMetrics())}
}

func internalMux(m *Metrics) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", m.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok\n")
	})
	return mux
}

// NewPublicServer is the public listener with the bounding timeouts.
func NewPublicServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: h,
		ReadHeaderTimeout: readHeaderTimeout, ReadTimeout: readTimeout,
		WriteTimeout: writeTimeout, IdleTimeout: idleTimeout,
	}
}

// NewInternalServer is the probe and metrics listener.
func NewInternalServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: h,
		ReadHeaderTimeout: readHeaderTimeout, ReadTimeout: readTimeout,
		WriteTimeout: readTimeout, IdleTimeout: idleTimeout,
	}
}
