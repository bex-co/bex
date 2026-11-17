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

package core

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/m450: a user agent is stored bounded and always storable. Postgres refuses
// a text value that is not UTF-8, and the sink swallows that error, so an
// unsanitized header would let a caller drop their own audit rows.
func TestAuditUserAgentIsBoundedValidText(t *testing.T) {
	chrome := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	if got := AuditUserAgent(chrome); got != chrome {
		t.Errorf("an ordinary browser agent changed: %q", got)
	}
	if got := AuditUserAgent(strings.Repeat("a", 2000)); len(got) != MaxAuditUserAgentBytes {
		t.Errorf("long agent kept %d bytes, want %d", len(got), MaxAuditUserAgentBytes)
	}
	// A three-byte rune straddling the limit is dropped whole, never split.
	straddling := strings.Repeat("a", MaxAuditUserAgentBytes-1) + "€tail"
	if got := AuditUserAgent(straddling); got != strings.Repeat("a", MaxAuditUserAgentBytes-1) || !utf8.ValidString(got) {
		t.Errorf("straddling rune: kept %d bytes, valid=%v", len(got), utf8.ValidString(got))
	}
	if got := AuditUserAgent("curl/8\xff\xfe.4"); !utf8.ValidString(got) || got != "curl/8\uFFFD.4" {
		t.Errorf("invalid UTF-8 = %q, want it replaced", got)
	}
	if got := AuditUserAgent("evil\x00agent\x1b[2J"); got != "evilagent[2J" {
		t.Errorf("control characters = %q, want them removed", got)
	}
}

func auditOriginCtx(subject, method string) context.Context {
	ctx := WithRequestOrigin(context.Background(), RequestOrigin{Transport: TransportGraphQL, UserAgent: "Mozilla/5.0", SourceIP: "198.51.100.23"})
	if subject == "" {
		return ctx
	}
	return WithIdentity(ctx, Identity{Subject: subject, Method: method})
}

// A row the actor's own request caused carries the request's address and
// agent; a system row, a row naming someone else, and a row written outside a
// request carry neither, and a writer's own values are never overwritten.
func TestAttachRequestOriginOnlyForTheRequestersOwnRows(t *testing.T) {
	cases := []struct {
		name           string
		ctx            context.Context
		ev             AuditEvent
		wantIP, wantUA string
	}{
		{"the requester's row", auditOriginCtx("kratos-a", "session"), AuditEvent{Caller: "kratos-a"}, "198.51.100.23", "Mozilla/5.0"},
		{"an API key's row", auditOriginCtx("client-1", "oauth2"), AuditEvent{Caller: "client-1"}, "198.51.100.23", "Mozilla/5.0"},
		{"a system row inside a request", auditOriginCtx("kratos-a", "session"), AuditEvent{}, "", ""},
		{"a row naming another actor", auditOriginCtx("kratos-a", "session"), AuditEvent{Caller: "kratos-b"}, "", ""},
		{"an unauthenticated request", auditOriginCtx("", ""), AuditEvent{Caller: "kratos-a"}, "", ""},
		{"no request at all", WithIdentity(context.Background(), Identity{Subject: "kratos-a"}), AuditEvent{Caller: "kratos-a"}, "", ""},
		{"a writer's own values", auditOriginCtx("kratos-a", "session"), AuditEvent{Caller: "kratos-a", SourceIP: "203.0.113.1", UserAgent: "OpenSSH_9.6"}, "203.0.113.1", "OpenSSH_9.6"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := tc.ev
			attachRequestOrigin(tc.ctx, &ev)
			if ev.SourceIP != tc.wantIP || ev.UserAgent != tc.wantUA {
				t.Errorf("got %q / %q, want %q / %q", ev.SourceIP, ev.UserAgent, tc.wantIP, tc.wantUA)
			}
		})
	}
}

// Every sink path a Base writes through records the origin: an authorization
// row (emit), a typed effect row (recordAudit) and a batch (recordAudits), and
// the stored agent is the bounded one.
func TestEveryAuditPathRecordsTheRequestOrigin(t *testing.T) {
	ctx := WithIdentity(WithRequestOrigin(context.Background(), RequestOrigin{
		Transport: TransportREST, UserAgent: strings.Repeat("x", 600), SourceIP: "2001:db8::7",
	}), Identity{Subject: "client-1", Method: "oauth2"})
	sink := &batchAuditSink{}
	b := &Base{Audit: sink, Authz: &fakeAllowChecker{}}
	if err := scaleLikeVerb(ctx, b, "web"); err != nil {
		t.Fatal(err)
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
	b.RecordScaleEffect(ctx, app, 1, 2)
	b.RecordPlanChanged(ctx, app, "starter", "free", new(false))
	rows := append(append([]AuditEvent{}, sink.events...), sink.batches[0]...)
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want the authorization, the scale effect and the plan batch of two", len(rows))
	}
	for _, ev := range rows {
		if ev.SourceIP != "2001:db8::7" || len(ev.UserAgent) != MaxAuditUserAgentBytes {
			t.Errorf("%s: source %q, agent %d bytes", ev.Verb, ev.SourceIP, len(ev.UserAgent))
		}
	}
}
