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

// Command objectgw is the public bucket gateway (ADR112 D4): it serves the
// objects of buckets a workspace published with `wrangler r2 bucket dev-url
// enable`, read-only and with no credential, at pub-<xid>.<BEX_BASE_DOMAIN>.
//
// Its environment is credentials and nothing else (ADR112 D9): a read-only
// Wasabi identity and the bearer it speaks to bex-api with. Without them it
// stays up and answers 503, so a cluster that has no Wasabi does not
// crash-loop. Killing public hosting is scaling it to zero in GitOps
// (docs/runbooks/objectgw-kill-switch.md).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectgw"
)

// shutdownGrace is how long in-flight responses get to finish after a stop. A
// large object going to a slow reader is cut at this point, which is inside the
// pod's termination grace period.
const shutdownGrace = 25 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	creds := objectgw.Credentials{
		ReadAccessKey: strings.TrimSpace(os.Getenv("BEX_OBJECT_STORAGE_READ_ACCESS_KEY")),
		ReadSecretKey: strings.TrimSpace(os.Getenv("BEX_OBJECT_STORAGE_READ_SECRET_KEY")),
		Token:         strings.TrimSpace(os.Getenv("BEX_OBJECT_STORAGE_GATEWAY_TOKEN")),
	}
	var gw *objectgw.Gateway
	if missing := creds.Missing(); len(missing) > 0 {
		log.Printf("objectgw: %s not set; serving 503 until the bex-system/bex-object-storage-read Secret is installed", strings.Join(missing, ", "))
		gw = objectgw.NewDegraded()
	} else {
		gw = objectgw.New(objectgw.Config{Credentials: creds})
		go gw.Meter.Run(ctx, objectgw.ReportInterval)
	}

	public := objectgw.NewPublicServer(objectgw.PublicAddr, gw.Public)
	internal := objectgw.NewInternalServer(objectgw.InternalAddr, gw.Internal)
	errs := make(chan error, 2)
	for _, srv := range []*http.Server{public, internal} {
		go func() { errs <- srv.ListenAndServe() }()
	}
	log.Printf("objectgw: public %s, internal %s", objectgw.PublicAddr, objectgw.InternalAddr)

	select {
	case <-ctx.Done():
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("objectgw: serve: %v", err)
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	for _, srv := range []*http.Server{public, internal} {
		if err := srv.Shutdown(shutdown); err != nil {
			log.Printf("objectgw: shutdown: %v", err)
		}
	}
	// Once the last responses are out: what they sent is reported, so a clean
	// stop loses nothing.
	if gw.Meter != nil {
		final, cancelFinal := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelFinal()
		if err := gw.Meter.Flush(final); err != nil {
			log.Printf("objectgw: final egress report: %v", err)
		}
	}
}
