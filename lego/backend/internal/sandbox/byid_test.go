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

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// byIDKeys is a tenant-key provider with the lookup-only seam. It records
// every mint so the test can prove routing never mints a key.
type byIDKeys struct {
	mu     sync.Mutex
	keys   map[string]string // workspace -> existing key
	minted []string
}

func (k *byIDKeys) WorkspaceKey(_ context.Context, ws string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.minted = append(k.minted, ws)
	if key, ok := k.keys[ws]; ok {
		return key, nil
	}
	return "key-" + ws, nil
}

func (k *byIDKeys) SandboxKeyLookup(_ context.Context, ws string) (string, bool, error) {
	key, ok := k.keys[ws]
	return key, ok, nil
}

// byIDFixture serves one OpenSandbox namespace per tenant key: key-tea-a holds
// os-a (tea-a), key-tea-b holds os-b (sbx-b) (tea-b), key-tea-x holds os-x
// (tea-x). Every sandbox is owned by id-a.
func byIDFixture(t *testing.T) (*Service, *byIDKeys, *[]string) {
	t.Helper()
	namespaces := map[string][]osSandbox{}
	for _, ws := range []string{"tea-a", "tea-b", "tea-x"} {
		suffix := ws[len(ws)-1:]
		row := osSandbox{
			ID: "os-" + suffix,
			Metadata: map[string]string{
				metadataOwner: "id-a", metadataWorkspace: ws, metadataRegime: metadataSandboxRegime,
				metadataNetworkPolicy: string(NetworkPolicyDenyAll), metadataPublicID: "sbx-" + suffix,
			},
		}
		row.Status.State = "Running"
		namespaces["key-"+ws] = []osSandbox{row}
	}
	var mu sync.Mutex
	var deletes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rows := namespaces[r.Header.Get(tenantKeyHeader)]
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			_ = json.NewEncoder(w).Encode(rows)
			return
		case r.Method == http.MethodGet || r.Method == http.MethodDelete:
			for _, row := range rows {
				if r.URL.Path != "/sandboxes/"+row.ID {
					continue
				}
				if r.Method == http.MethodDelete {
					mu.Lock()
					deletes = append(deletes, row.ID)
					mu.Unlock()
					w.WriteHeader(http.StatusNoContent)
					return
				}
				_ = json.NewEncoder(w).Encode(row)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	// tea-c has never created a sandbox, so it has no tenant key: the probe
	// passes it on the way to tea-b, and must not mint one there.
	members := coretest.Members{"id-a": {"tea-a", "tea-c", "tea-b"}, "id-x": {"tea-x"}}
	keys := &byIDKeys{keys: map[string]string{"tea-a": "key-tea-a", "tea-b": "key-tea-b", "tea-x": "key-tea-x"}}
	svc := &Service{
		Base: &core.Base{
			Namespace: "default",
			Workspace: members,
			Authz:     adminChecker{},
			MemberWorkspaceIDs: func(_ context.Context, id core.Identity) ([]string, error) {
				return members[id.Subject], nil
			},
		},
		Client: NewClient(srv.URL),
		Keys:   keys,
		Exec: &ExecConfig{
			Secret: []byte("byid-secret"), GatewayURL: "http://gateway.invalid/sandbox-exec",
			FileGatewayURL: "http://gateway.invalid/sandbox-files",
			Nonces:         &sshgateway.NonceGuard{Store: &gatewaytest.FakeStore{}},
		},
	}
	return svc, keys, &deletes
}

// TestByIDVerbsReachNonDefaultWorkspaceSandbox (w4/m172): a member of
// [tea-a (default), tea-b] reaches a tea-b sandbox by id with no ownerId on
// every by-id verb, and the routing read never mints a tenant key.
func TestByIDVerbsReachNonDefaultWorkspaceSandbox(t *testing.T) {
	svc, keys, deletes := byIDFixture(t)
	ctx := identityCtx("id-a")

	for _, id := range []string{"os-b", "sbx-b"} {
		got, err := svc.Get(ctx, id)
		if err != nil || got.Workspace != "tea-b" || got.ID != "sbx-b" {
			t.Fatalf("Get(%s) = %+v, %v; want the tea-b sandbox", id, got, err)
		}
	}
	run, err := svc.ConnectRun(ctx, ConnectRequest{SandboxID: "sbx-b", Command: "true", Operation: ConnectOperationStream}, "https://api.example")
	if err != nil || run.Token == "" {
		t.Fatalf("ConnectRun = %+v, %v; want a token minted for the tea-b sandbox", run, err)
	}
	file, err := svc.ConnectFile(ctx, FileConnectRequest{SandboxID: "sbx-b", Operation: sandboxfiles.OperationDownload, Path: "/tmp/out.txt"}, "https://api.example")
	if err != nil || file.Token == "" {
		t.Fatalf("ConnectFile = %+v, %v; want a token minted for the tea-b sandbox", file, err)
	}
	var claims fileConnectClaims
	if err := fileConnectCodec.Open(svc.Exec.fileConnectKey(), file.Token, &claims); err != nil || claims.Workspace != "tea-b" {
		t.Fatalf("file token claims = %+v, %v; want workspace tea-b", claims, err)
	}
	if err := svc.Terminate(ctx, "sbx-b"); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	if !slices.Equal(*deletes, []string{"os-b"}) {
		t.Fatalf("deletes = %v, want [os-b]", *deletes)
	}
	// The verb's own scoped path resolves the EXISTING tea-b key; routing never
	// minted one for tea-c, the member workspace it probed that has none.
	if slices.Contains(keys.minted, "tea-c") {
		t.Fatalf("minted a key for tea-c while routing: %v", keys.minted)
	}
}

// TestByIDVerbsForeignAndMissingIDsAreIdenticalNotFound: a sandbox in a
// workspace the caller does not belong to answers exactly the not-found a
// nonexistent id does, on reads, lifecycle, and token mints alike.
func TestByIDVerbsForeignAndMissingIDsAreIdenticalNotFound(t *testing.T) {
	svc, _, deletes := byIDFixture(t)
	ctx := identityCtx("id-a")

	type verb func(id string) error
	verbs := map[string]verb{
		"Get":       func(id string) error { _, err := svc.Get(ctx, id); return err },
		"Suspend":   func(id string) error { return svc.Suspend(ctx, id) },
		"Terminate": func(id string) error { return svc.Terminate(ctx, id) },
		"ConnectRun": func(id string) error {
			_, err := svc.ConnectRun(ctx, ConnectRequest{SandboxID: id, Command: "true", Operation: ConnectOperationStream}, "https://api.example")
			return err
		},
		"ConnectFile": func(id string) error {
			_, err := svc.ConnectFile(ctx, FileConnectRequest{SandboxID: id, Operation: sandboxfiles.OperationDownload, Path: "/tmp/out.txt"}, "https://api.example")
			return err
		},
	}
	for name, call := range verbs {
		for _, pair := range [][2]string{{"os-x", "os-missing"}, {"sbx-x", "sbx-missing"}} {
			foreign, missing := call(pair[0]), call(pair[1])
			if !errors.Is(foreign, core.ErrNotFound) || !errors.Is(missing, core.ErrNotFound) {
				t.Fatalf("%s: foreign=%v missing=%v; want not-found for both", name, foreign, missing)
			}
			var fc, mc *core.CodedError
			if !errors.As(foreign, &fc) || !errors.As(missing, &mc) || fc.Code != mc.Code || fc.Code != "SANDBOX_NOT_FOUND" {
				t.Fatalf("%s: foreign=%#v missing=%#v; want identical SANDBOX_NOT_FOUND", name, foreign, missing)
			}
		}
	}
	if len(*deletes) != 0 {
		t.Fatalf("foreign terminate issued deletes %v", *deletes)
	}
}

// TestByIDExplicitOwnerKeepsScopedBehavior: a named workspace (ownerId) is
// honored as before — the sandbox is looked up only there, so a mismatched one
// is not-found and a correct one resolves.
func TestByIDExplicitOwnerKeepsScopedBehavior(t *testing.T) {
	svc, _, _ := byIDFixture(t)
	inA := core.WithWorkspace(identityCtx("id-a"), "tea-a")
	if _, err := svc.Get(inA, "sbx-b"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get with mismatched ownerId = %v, want not-found", err)
	}
	if err := svc.Suspend(inA, "os-b"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Suspend with mismatched ownerId = %v, want not-found", err)
	}
	if _, err := svc.ConnectRun(identityCtx("id-a"), ConnectRequest{OwnerID: "tea-a", SandboxID: "sbx-b", Command: "true", Operation: ConnectOperationStream}, "https://api.example"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("ConnectRun with mismatched ownerId = %v, want not-found", err)
	}
	if _, err := svc.ConnectFile(identityCtx("id-a"), FileConnectRequest{OwnerID: "tea-a", SandboxID: "sbx-b", Operation: sandboxfiles.OperationDownload, Path: "/tmp/out.txt"}, "https://api.example"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("ConnectFile with mismatched ownerId = %v, want not-found", err)
	}
	if got, err := svc.Get(core.WithWorkspace(identityCtx("id-a"), "tea-b"), "sbx-b"); err != nil || got.Workspace != "tea-b" {
		t.Fatalf("Get with matching ownerId = %+v, %v", got, err)
	}
}
