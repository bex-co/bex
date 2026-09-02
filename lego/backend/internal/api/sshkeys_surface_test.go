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

package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type surfaceSSHKeyStore struct {
	keys map[string]store.SSHKey
}

func (s *surfaceSSHKeyStore) CreateSSHKey(_ context.Context, key store.SSHKey) (store.SSHKey, error) {
	for _, existing := range s.keys {
		if existing.Fingerprint == key.Fingerprint {
			return store.SSHKey{}, store.ErrConflict
		}
	}
	key.CreatedAt = time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	s.keys[key.ID] = key
	return key, nil
}
func (s *surfaceSSHKeyStore) ListSSHKeys(_ context.Context, subject string) ([]store.SSHKey, error) {
	var out []store.SSHKey
	for _, key := range s.keys {
		if key.Subject == subject {
			out = append(out, key)
		}
	}
	return out, nil
}
func (s *surfaceSSHKeyStore) DeleteSSHKey(_ context.Context, subject, id string) error {
	if key, ok := s.keys[id]; !ok || key.Subject != subject {
		return store.ErrNotFound
	}
	delete(s.keys, id)
	return nil
}
func (s *surfaceSSHKeyStore) SSHKeyByFingerprint(_ context.Context, fingerprint string) (store.SSHKey, error) {
	for _, key := range s.keys {
		if key.Fingerprint == fingerprint {
			return key, nil
		}
	}
	return store.SSHKey{}, store.ErrNotFound
}

func surfacePublicKey(t *testing.T) string {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}

func rawGraphQL(t *testing.T, h http.Handler, query string) (map[string]any, []any, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"query": query})
	response := do(t, h, http.MethodPost, "/graphql", testToken, string(body))
	var envelope struct {
		Data   map[string]any `json:"data"`
		Errors []any          `json:"errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data, envelope.Errors, response.Body.String()
}

func TestSSHKeysRESTGraphQLMCPParity(t *testing.T) {
	st := &surfaceSSHKeyStore{keys: map[string]store.SSHKey{}}
	base := &core.Base{Client: fakeClient(), Namespace: "default"}
	srv := NewServer(base, Deps{SSHKeysStore: st})
	// round-7 F3: create enrolls a durable credential, so only a mint-eligible
	// caller class succeeds — the parity test rides a human token from a
	// registry-listed platform client (fakeHumanHydra); a machine or third-party token
	// gets 403 on all three surfaces alike.
	srv.HydraAdminURL = fakeHumanHydra(t).URL
	srv.OAuthPlatformClients = []string{"platform-cli"}
	h := buildHandler(t, srv)
	restPublicKey, gqlPublicKey, mcpPublicKey := surfacePublicKey(t), surfacePublicKey(t), surfacePublicKey(t)

	body, _ := json.Marshal(map[string]string{"name": "rest", "publicKey": restPublicKey})
	rest := do(t, h, http.MethodPost, "/v1/ssh-keys", testToken, string(body))
	if rest.Code != http.StatusCreated {
		t.Fatalf("REST create = %d %s", rest.Code, rest.Body.String())
	}
	var created store.SSHKey
	if err := json.Unmarshal(rest.Body.Bytes(), &created); err != nil || created.Name != "rest" {
		t.Fatalf("REST create decode = %+v, %v", created, err)
	}
	gqlData := gql(t, h, fmt.Sprintf(`mutation { createSSHKey(name: "gql", publicKey: %q) { id name publicKey fingerprint createdAt } }`, gqlPublicKey))
	gqlCreated := gqlData["createSSHKey"].(map[string]any)
	if gqlCreated["name"] != "gql" || gqlCreated["fingerprint"] == "" {
		t.Fatalf("GraphQL create = %+v", gqlCreated)
	}
	client := mcpSessionAs(t, srv, "identity-1")
	mcpCreated := callTool[store.SSHKey](t, client, "add_ssh_key", map[string]any{"name": "mcp", "publicKey": mcpPublicKey})
	if mcpCreated.Name != "mcp" || mcpCreated.Fingerprint == "" {
		t.Fatalf("MCP create = %+v", mcpCreated)
	}

	restList := do(t, h, http.MethodGet, "/v1/ssh-keys", testToken, "")
	var restKeys []store.SSHKey
	if err := json.Unmarshal(restList.Body.Bytes(), &restKeys); err != nil || len(restKeys) != 3 {
		t.Fatalf("REST list = %+v, %v", restKeys, err)
	}
	gqlData = gql(t, h, `{ sshKeys { id name publicKey fingerprint createdAt } }`)
	gqlKeys := gqlData["sshKeys"].([]any)
	if len(gqlKeys) != 3 {
		t.Fatalf("GraphQL list = %+v", gqlKeys)
	}
	mcpKeys := callTool[struct{ SSHKeys []store.SSHKey }](t, client, "list_ssh_keys", nil)
	if len(mcpKeys.SSHKeys) != 3 {
		t.Fatalf("MCP list = %+v", mcpKeys)
	}

	if response := do(t, h, http.MethodDelete, "/v1/ssh-keys/"+created.ID, testToken, ""); response.Code != http.StatusNoContent {
		t.Fatalf("REST delete = %d", response.Code)
	}
	if deleted := gql(t, h, fmt.Sprintf(`mutation { deleteSSHKey(id: %q) }`, gqlCreated["id"]))["deleteSSHKey"]; deleted != true {
		t.Fatalf("GraphQL delete = %v", deleted)
	}
	if deleted := callTool[struct{ Deleted bool }](t, client, "delete_ssh_key", map[string]any{"id": mcpCreated.ID}); !deleted.Deleted {
		t.Fatalf("MCP delete = %+v", deleted)
	}
	if keys := callTool[struct{ SSHKeys []store.SSHKey }](t, client, "list_ssh_keys", nil); len(keys.SSHKeys) != 0 {
		t.Fatalf("keys remain after three-surface deletion: %+v", keys)
	}

	invalidBody, _ := json.Marshal(map[string]string{"name": "bad", "publicKey": "private key text"})
	if response := do(t, h, http.MethodPost, "/v1/ssh-keys", testToken, string(invalidBody)); response.Code != http.StatusBadRequest {
		t.Fatalf("REST malformed = %d", response.Code)
	}
	_, gqlErrors, _ := rawGraphQL(t, h, `mutation { createSSHKey(name: "bad", publicKey: "private key text") { id } }`)
	if len(gqlErrors) == 0 {
		t.Fatal("GraphQL malformed key succeeded")
	}
	callToolError(t, client, "add_ssh_key", map[string]any{"name": "bad", "publicKey": "private key text"})

	duplicateBody, _ := json.Marshal(map[string]string{"name": "first", "publicKey": restPublicKey})
	first := do(t, h, http.MethodPost, "/v1/ssh-keys", testToken, string(duplicateBody))
	if first.Code != http.StatusCreated {
		t.Fatalf("duplicate fixture create = %d", first.Code)
	}
	_, gqlErrors, _ = rawGraphQL(t, h, fmt.Sprintf(`mutation { createSSHKey(name: "duplicate", publicKey: %q) { id } }`, restPublicKey))
	if len(gqlErrors) == 0 {
		t.Fatal("GraphQL duplicate key succeeded")
	}
	callToolError(t, client, "add_ssh_key", map[string]any{"name": "duplicate", "publicKey": restPublicKey})

	foreignID := ids.New(ids.SSHKey)
	st.keys[foreignID] = store.SSHKey{ID: foreignID, Subject: "foreign-subject", Name: "foreign", PublicKey: surfacePublicKey(t), Fingerprint: "SHA256:foreign"}
	for name, id := range map[string]string{"unknown": ids.New(ids.SSHKey), "foreign": foreignID} {
		t.Run(name+" delete", func(t *testing.T) {
			response := do(t, h, http.MethodDelete, "/v1/ssh-keys/"+id, testToken, "")
			if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "foreign-subject") {
				t.Fatalf("REST %s delete = %d %s", name, response.Code, response.Body.String())
			}
			_, errors, gqlBody := rawGraphQL(t, h, fmt.Sprintf(`mutation { deleteSSHKey(id: %q) }`, id))
			if len(errors) == 0 || strings.Contains(gqlBody, "foreign-subject") {
				t.Fatalf("GraphQL %s delete leaked/succeeded: %s", name, gqlBody)
			}
			callToolError(t, client, "delete_ssh_key", map[string]any{"id": id})
		})
	}
}

func TestSSHAddressIsNotAdvertisedWithoutKeyEnrollmentStore(t *testing.T) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelAppID: "srv-d5t5d4v8g3c73f5m9peg"}
	app.Spec.Tier = "starter"
	app.Status.Image = app.Spec.Image
	app.Status.ActiveRevision = "rev-1"
	base := &core.Base{Client: fakeClient(app), Namespace: "default"}

	withoutStore := NewServer(base, Deps{SSHHost: "ssh.bex.co"})
	view, err := withoutStore.Apps.Get(context.Background(), app.Labels[core.LabelAppID])
	if err != nil {
		t.Fatal(err)
	}
	if view.SSHAddress != "" {
		t.Fatalf("address advertised without key store: %q", view.SSHAddress)
	}

	withStore := NewServer(base, Deps{
		SSHHost:      "ssh.bex.co",
		SSHKeysStore: &surfaceSSHKeyStore{keys: map[string]store.SSHKey{}},
	})
	view, err = withStore.Apps.Get(context.Background(), app.Labels[core.LabelAppID])
	if err != nil {
		t.Fatal(err)
	}
	if view.SSHAddress != app.Labels[core.LabelAppID]+"@ssh.bex.co" {
		t.Fatalf("address with key store = %q", view.SSHAddress)
	}
}
