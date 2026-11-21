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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/postgres"
)

func TestParseLogicalReplicationArgs(t *testing.T) {
	parse := func(args ...string) (postgres.LogicalReplicationRequest, error) {
		return parseLogicalReplicationArgs(args, strings.NewReader(""))
	}
	req, err := parse("enable", "--database", "dpg-1", "--role", "cdc_reader", "--schema", "public", "--schema", "sales", "--all-tables")
	if err != nil {
		t.Fatal(err)
	}
	if !req.Enable || req.Database != "dpg-1" || req.PublisherRole != "cdc_reader" || strings.Join(req.Schemas, ",") != "public,sales" || !req.AllTables {
		t.Errorf("enable = %+v", req)
	}
	req, err = parse("disable", "--database", "dpg-1")
	if err != nil || req.Enable || req.Database != "dpg-1" || req.Publisher || req.Subscriber || req.Subscription != nil {
		t.Errorf("disable = %+v, %v", req, err)
	}
	// The subscriber forms (w34/m542).
	req, err = parse("enable", "--database", "dpg-1", "--subscriber-role", "orders_user")
	if err != nil || req.SubscriberRole != "orders_user" || req.PublisherRole != "" || req.Subscription != nil {
		t.Errorf("enable a subscriber role = %+v, %v", req, err)
	}
	// The URL carries a password, so it comes from stdin, never the command
	// line.
	req, err = parseLogicalReplicationArgs([]string{"enable", "--database", "dpg-1", "--subscription", "from_render", "--publisher-url", "-", "--publication", "pub"},
		strings.NewReader("postgresql://u:secret@h/db\n"))
	if err != nil || req.Subscription == nil || *req.Subscription != (postgres.LogicalSubscriptionRequest{Name: "from_render", PublisherURL: "postgresql://u:secret@h/db", Publication: "pub"}) {
		t.Errorf("enable a subscription = %+v, %v", req.Subscription, err)
	}
	if _, err := parseLogicalReplicationArgs([]string{"enable", "--database", "dpg-1", "--subscription", "s", "--publisher-url", "-", "--publication", "pub"},
		strings.NewReader(strings.Repeat("x", publisherURLMaxBytes+1))); err == nil {
		t.Error("an oversized stdin URL was accepted")
	}
	req, err = parse("disable", "--database", "dpg-1", "--subscriber")
	if err != nil || !req.Subscriber || req.Publisher {
		t.Errorf("disable the subscriber = %+v, %v", req, err)
	}
	req, err = parse("disable", "--database", "dpg-1", "--subscription", "from_render")
	if err != nil || req.Subscription == nil || req.Subscription.Name != "from_render" || req.Subscription.PublisherURL != "" {
		t.Errorf("remove a subscription = %+v, %v", req, err)
	}
	for _, bad := range [][]string{
		nil,
		{"enable"},
		{"enable", "--database", "dpg-1"}, // nothing to enable
		{"enable", "--database", "dpg-1", "--schema", "public"},                                              // no role
		{"enable", "--database", "dpg-1", "--role", "cdc_reader"},                                            // no schema
		{"enable", "--database", "dpg-1", "--all-tables"},                                                    // no role
		{"enable", "--database", "srv-1", "--role", "r", "--schema", "p"},                                    // not a Postgres
		{"enable", "--database", "dpg-1", "--subscription", "s", "--publication", "pub"},                     // no URL
		{"enable", "--database", "dpg-1", "--subscription", "s", "--publisher-url", "postgresql://u:p@h/db"}, // no publication
		{"enable", "--database", "dpg-1", "--publisher-url", "-", "--publication", "pub"},                    // no name
		{"disable", "--database", "dpg-1", "--role", "cdc_reader"},                                           // enable's flags
		{"disable", "--database", "dpg-1", "--subscriber-role", "orders_user"},                               // enable's flags
		{"disable", "--database", "dpg-1", "--subscription", "s", "--subscriber"},                            // one or the other
		{"disable", "dpg-1"},
		{"toggle", "--database", "dpg-1"},
	} {
		if _, err := parse(bad...); err == nil {
			t.Errorf("parse %q: want a usage error", bad)
		}
	}
}

// The command exits 0 only once the database reports the change, 3 while it
// has not yet, and 1 on a refusal or an answer it cannot read, printing the
// verb's answer each time.
func TestRunLogicalReplicationExitCodes(t *testing.T) {
	answer := postgres.LogicalReplicationResult{Database: "dpg-1", Converged: true}
	status, raw := http.StatusOK, ""
	var got postgres.LogicalReplicationRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != postgres.LogicalReplicationPath || r.Header.Get("Authorization") != "Bearer cp-token" {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		if raw != "" {
			_, _ = w.Write([]byte(raw))
			return
		}
		_ = json.NewEncoder(w).Encode(answer)
	}))
	defer srv.Close()
	cfg := &Config{CPToken: "cp-token", CPAddr: strings.TrimPrefix(srv.URL, "http://")}
	run := func() (int, string) {
		var out, errOut bytes.Buffer
		code := runLogicalReplication(context.Background(), cfg, []string{"disable", "--database", "dpg-1"}, strings.NewReader(""), &out, &errOut)
		return code, out.String()
	}
	if code, out := run(); code != 0 || !strings.Contains(out, `"converged":true`) || got.Database != "dpg-1" || got.Enable {
		t.Errorf("converged: exit %d, output %q, request %+v", code, out, got)
	}
	answer.Converged = false
	if code, _ := run(); code != 3 {
		t.Errorf("not converged: exit %d, want 3", code)
	}
	raw = "<html>proxy error</html>"
	if code, _ := run(); code != 1 {
		t.Errorf("an answer that does not parse: exit %d, want 1", code)
	}
	raw, status = "", http.StatusBadRequest
	if code, _ := run(); code != 1 {
		t.Errorf("refused: exit %d, want 1", code)
	}
	cfg.CPToken = ""
	if code, _ := run(); code != 2 {
		t.Errorf("no control-plane token: exit %d, want 2", code)
	}
}
