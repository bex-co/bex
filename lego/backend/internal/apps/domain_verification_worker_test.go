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

package apps

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeDomainResolver scripts the two lookups the traffic predicate runs.
type fakeDomainResolver struct {
	cnames map[string]string
	addrs  map[string][]string
	err    error
}

func (f fakeDomainResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if cname, ok := f.cnames[host]; ok {
		return cname, nil
	}
	return host, nil
}

func (f fakeDomainResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.addrs[host], nil
}

func TestTrafficPredicate(t *testing.T) {
	const platform = "web.onbex.co"
	cases := []struct {
		name     string
		host     string
		resolver fakeDomainResolver
		want     bool
	}{
		{
			name: "direct CNAME to the platform host",
			host: "www.example.com",
			resolver: fakeDomainResolver{cnames: map[string]string{
				"www.example.com": "web.onbex.co.",
			}},
			want: true,
		},
		{
			name: "multi-hop chain through the platform host",
			host: "www.example.com",
			resolver: fakeDomainResolver{cnames: map[string]string{
				"www.example.com": "edge.onbex.co",
				"web.onbex.co":    "edge.onbex.co",
			}},
			want: true,
		},
		{
			name: "apex flattened to the edge addresses",
			host: "example.com",
			resolver: fakeDomainResolver{addrs: map[string][]string{
				"example.com":  {"49.12.20.236"},
				"web.onbex.co": {"49.12.20.236"},
			}},
			want: true,
		},
		{
			name: "apex A equal to one edge address among several",
			host: "example.com",
			resolver: fakeDomainResolver{addrs: map[string][]string{
				"example.com":  {"49.12.20.236", "203.0.113.9"},
				"web.onbex.co": {"49.12.20.236"},
			}},
			want: true,
		},
		{
			name: "CNAME to a different service's platform host",
			host: "www.example.com",
			resolver: fakeDomainResolver{cnames: map[string]string{
				"www.example.com": "other.onbex.co",
			}, addrs: map[string][]string{
				"www.example.com": {"198.51.100.7"},
				"web.onbex.co":    {"49.12.20.236"},
			}},
			want: false,
		},
		{
			name: "Cloudflare-proxied record resolves to anycast, not the edge",
			host: "example.com",
			resolver: fakeDomainResolver{addrs: map[string][]string{
				"example.com":  {"104.21.62.201", "172.67.138.197"},
				"web.onbex.co": {"49.12.20.236"},
			}},
			want: false,
		},
		{
			name:     "no records at all",
			host:     "www.example.com",
			resolver: fakeDomainResolver{},
			want:     false,
		},
		{
			name:     "resolver error",
			host:     "www.example.com",
			resolver: fakeDomainResolver{err: errors.New("SERVFAIL")},
			want:     false,
		},
		{
			name: "the platform host itself is not a custom domain",
			host: "web.onbex.co",
			resolver: fakeDomainResolver{addrs: map[string][]string{
				"web.onbex.co": {"49.12.20.236"},
			}},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checker := systemDomainTrafficChecker{resolver: tc.resolver}
			if got := checker.PointsAtService(context.Background(), tc.host, platform); got != tc.want {
				t.Errorf("PointsAtService(%q, %q) = %v, want %v", tc.host, platform, got, tc.want)
			}
		})
	}
}

// The sweep alone — no Verify call — promotes a claim whose traffic record
// has landed, projects it, and records exactly one verified event; a
// never-pointed claim stays unverified with bounded attempts.
func TestDomainVerificationSweepPromotesWithoutAClick(t *testing.T) {
	svc, claims, sink := auditedDomainService(t)
	worker := &DomainVerificationWorker{Svc: svc}
	ctx := context.Background()
	for _, host := range []string{"app.example.com", "never.example.com"} {
		if _, err := svc.AddDomain(ctx, "web", host); err != nil {
			t.Fatalf("AddDomain(%s): %v", host, err)
		}
	}
	svc.DomainTraffic = domainTrafficFunc(func(_ context.Context, host, _ string) bool {
		return host == "app.example.com"
	})

	if err := worker.tick(ctx); err != nil {
		t.Fatalf("tick: %v", err)
	}
	promoted, err := claims.GetDomainClaim(ctx, "srv-1", "app.example.com")
	if err != nil || promoted.ClaimState != "verified" {
		t.Fatalf("pointed claim after sweep = %+v err=%v, want verified", promoted, err)
	}
	// auditedDomainService's client is behind newService; fetch through ListDomains.
	views, err := svc.ListDomains(ctx, "web")
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	byName := map[string]DomainView{}
	for _, v := range views {
		byName[v.Name] = v
	}
	if byName["app.example.com"].VerificationStatus != "verified" {
		t.Fatalf("sweep-promoted view = %+v", byName["app.example.com"])
	}
	if byName["never.example.com"].VerificationStatus != "unverified" {
		t.Fatalf("never-pointed view = %+v, want unverified", byName["never.example.com"])
	}
	if n := sink.countVerb(core.AuditVerbVerifyDomain); n != 1 {
		t.Fatalf("sweep recorded %d verification events, want exactly 1", n)
	}
	never, err := claims.GetDomainClaim(ctx, "srv-1", "never.example.com")
	if err != nil || never.VerificationAttempts != 1 {
		t.Fatalf("never-pointed attempts = %+v err=%v, want 1 recorded attempt", never, err)
	}

	// A second tick is a no-op for the verified claim (no second event) and
	// keeps the unpointed one unverified.
	if err := worker.tick(ctx); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbVerifyDomain); n != 1 {
		t.Fatalf("second tick recorded %d events total, want still 1", n)
	}
	if hosts := servingHosts(t, svc); !slices.Equal(hosts, []string{"app.example.com"}) {
		t.Fatalf("serving hosts = %v, want only the pointed domain", hosts)
	}
}

func servingHosts(t *testing.T, svc *Service) []string {
	t.Helper()
	// The projected serving spec is what ListDesiredApps would emit; through
	// the service surface, the verified claim's projection shows on the App
	// the sweep patched. Read it back through the fake client the fixture
	// built: GetDomain's view only proves state, so check the CR directly.
	app, err := svc.GetApp(context.Background(), core.RelCanView, "web")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}
	return app.Spec.Hosts
}
