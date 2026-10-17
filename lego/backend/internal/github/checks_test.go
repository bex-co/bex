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

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w1/m181: the checksPass reads.

func TestMintChecksTokenAsksForChecksAndReportsARefusal(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	granted := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Permissions map[string]string `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Permissions["checks"] != "read" || body.Permissions["metadata"] != "read" || len(body.Permissions) != 2 {
			t.Fatalf("checks token must ask for checks and metadata read only: %+v", body.Permissions)
		}
		if !granted {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"message":"The permissions requested are not granted to this installation."}`)
			return
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"ghs_checks","expires_at":"2026-10-17T13:00:00Z"}`)
	}))
	defer srv.Close()
	c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	c.baseURL = srv.URL

	tok, permitted, err := c.MintChecksToken(context.Background(), 7)
	if err != nil || !permitted || tok.Token != "ghs_checks" {
		t.Fatalf("granted = %+v %v %v", tok, permitted, err)
	}
	granted = false
	if _, permitted, err := c.MintChecksToken(context.Background(), 7); err != nil || permitted {
		t.Fatalf("refused = permitted %v err %v, want not permitted and no error", permitted, err)
	}
}

func TestListCheckRunsPages(t *testing.T) {
	keyPEM, _ := testKeyPEM(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/commits/abc/check-runs" || r.URL.Query().Get("filter") != "latest" || r.Header.Get("Authorization") != "token tok" {
			t.Errorf("unexpected request %s %s auth %q", r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"))
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		n := 100
		if page == 2 {
			n = 30
		}
		runs := make([]CheckRun, n)
		for i := range runs {
			runs[i] = CheckRun{Name: fmt.Sprintf("p%d-%d", page, i), Status: "completed", Conclusion: "success"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total_count": 130, "check_runs": runs})
	}))
	defer srv.Close()
	c, _ := NewClient(Config{AppID: "1", PrivateKey: keyPEM, Slug: "bex"})
	c.baseURL = srv.URL

	runs, err := c.ListCheckRuns(context.Background(), "tok", "o", "r", "abc")
	if err != nil || len(runs) != 130 || runs[129].Name != "p2-29" {
		t.Fatalf("runs = %d, %v", len(runs), err)
	}
}

// The seam mints from the connection that owns the repository's account and
// answers false, not an error, when the installation lacks Checks: read.
func TestChecksSourceUsesTheOwningConnection(t *testing.T) {
	fc := &fakeClient{token: "tok", installEvents: []string{"push", "check_suite"}, checkRuns: []CheckRun{{Name: "ci", Status: "completed", Conclusion: "success"}}}
	s := newChecksTestService(t, fc)
	src := s.DeployChecksSource()

	permitted, err := src.ChecksPermitted(context.Background(), "tea-a", "https://github.com/acme/app")
	if err != nil || !permitted {
		t.Fatalf("permitted = %v, %v", permitted, err)
	}
	runs, err := src.CommitCheckRuns(context.Background(), "tea-a", "https://github.com/acme/app", "abc")
	if err != nil || len(runs) != 1 || fc.gotChecksFor[0] != "checks-tok" || fc.gotChecksFor[1] != "acme" || fc.gotChecksFor[3] != "abc" {
		t.Fatalf("runs = %+v, %v (asked %v)", runs, err, fc.gotChecksFor)
	}
	if permitted, err := src.ChecksPermitted(context.Background(), "tea-a", "https://github.com/other/app"); err != nil || permitted {
		t.Errorf("unconnected owner = %v, %v, want false", permitted, err)
	}

	fc.installEvents = []string{"push"}
	if permitted, err := src.ChecksPermitted(context.Background(), "tea-a", "https://github.com/acme/app"); err != nil || permitted {
		t.Errorf("installation not receiving check_suite = %v, %v, want false", permitted, err)
	}
	fc.installEvents = []string{"push", "check_suite"}

	fc.checksDenied = true
	if permitted, err := src.ChecksPermitted(context.Background(), "tea-a", "https://github.com/acme/app"); err != nil || permitted {
		t.Errorf("installation without Checks: read = %v, %v, want false", permitted, err)
	}
	if _, err := src.CommitCheckRuns(context.Background(), "tea-a", "https://github.com/acme/app", "abc"); err == nil {
		t.Error("check runs without the permission must be an error")
	}
}

func newChecksTestService(t *testing.T, fc *fakeClient) *Service {
	t.Helper()
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "tea-a", InstallationID: 7, AccountLogin: "acme"})
	return &Service{Base: &core.Base{Namespace: "default"}, GitHub: fc, Store: st}
}
