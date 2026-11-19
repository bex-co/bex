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
	"errors"
	"fmt"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// config is the whole environment of bex-s3: credentials and the URLs of the
// services it authorizes with, read here and nowhere else (cmd/ is the only
// place that reads the environment). The endpoints, buckets, regions, ports and
// every limit are constants in internal/objectstorage and internal/s3gw
// (ADR112 D9).
type config struct {
	// DBURI is the control plane's Postgres (BEX_CP_DB_URI): buckets, access
	// keys, usage, audit rows and workspace membership.
	DBURI string
	// OpenFGAURL and OpenFGAToken authorize a request as its key's creator.
	OpenFGAURL, OpenFGAToken string
	// OpenBaoURL and OpenBaoJWTPath reach the access keys' secrets.
	OpenBaoURL, OpenBaoJWTPath string
	// KratosAdminURL reads a member's second factor and Google account for a
	// workspace's login requirements.
	KratosAdminURL string
	// TrustedProxies are the edge proxies (BEX_TRUSTED_PROXY_CIDRS, as bex-api reads
	// it) whose forwarding headers name the client a failed-authentication budget is
	// kept for. Unset trusts nobody: every request is keyed by its TCP peer, which
	// behind Traefik is one shared budget, so production sets it.
	TrustedProxies core.TrustedProxies
	// Namespace is bex-system, where the Base's own objects live.
	Namespace string
	// ObjectStorage is the read-write Wasabi pair.
	ObjectStorage objectstorage.Credentials
}

// loadConfig reads and validates the environment. Half a credential pair and a
// database with no OpenFGA are startup errors: the first would turn object
// storage off silently, the second would serve every request unauthorized.
func loadConfig(getenv func(string) string) (*config, error) {
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	cfg := &config{
		DBURI:          get("BEX_CP_DB_URI"),
		OpenFGAURL:     get("BEX_OPENFGA_URL"),
		OpenFGAToken:   get("BEX_OPENFGA_TOKEN"),
		OpenBaoURL:     get("BEX_OPENBAO_URL"),
		OpenBaoJWTPath: get("BEX_OPENBAO_JWT_PATH"),
		KratosAdminURL: get("BEX_KRATOS_ADMIN_URL"),
		Namespace:      get("BEX_NAMESPACE"),
		ObjectStorage: objectstorage.Credentials{
			AccessKey: get("BEX_OBJECT_STORAGE_ACCESS_KEY"),
			SecretKey: get("BEX_OBJECT_STORAGE_SECRET_KEY"),
		},
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "bex-system"
	}
	proxies, err := core.ParseTrustedProxies(get("BEX_TRUSTED_PROXY_CIDRS"))
	if err != nil {
		return nil, fmt.Errorf("BEX_TRUSTED_PROXY_CIDRS: %w", err)
	}
	cfg.TrustedProxies = proxies
	if missing := cfg.ObjectStorage.Missing(); len(missing) > 0 {
		return nil, errors.New(strings.Join(missing, ", ") + " is set without its pair")
	}
	if cfg.DBURI != "" && cfg.ObjectStorage.Enabled() && cfg.OpenFGAURL == "" {
		return nil, errors.New("BEX_CP_DB_URI is set but BEX_OPENFGA_URL is not: authorization would not be enforced")
	}
	return cfg, nil
}

// degradedReason names what keeps the surface from serving, or "" when it can.
// A degraded process stays up and Ready and answers 503, so a cluster without
// Wasabi, a control plane or OpenBao does not crash-loop.
func (c *config) degradedReason() string {
	switch {
	case !c.ObjectStorage.Enabled():
		return "BEX_OBJECT_STORAGE_ACCESS_KEY and BEX_OBJECT_STORAGE_SECRET_KEY are not set"
	case c.DBURI == "":
		return "BEX_CP_DB_URI is not set"
	case c.OpenBaoURL == "":
		return "BEX_OPENBAO_URL is not set (an access key's secret lives there)"
	}
	return ""
}
