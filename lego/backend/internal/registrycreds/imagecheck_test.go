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

package registrycreds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/graphql-go/graphql"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/registryread"
	"github.com/bex-co/bex/lego/backend/internal/registryread/registrytest"
)

// countingTransport counts the requests that reach the registry.
type countingTransport struct {
	next http.RoundTripper
	n    atomic.Int32
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n.Add(1)
	return c.next.RoundTrip(r)
}

func imageCheckService(t *testing.T) (*Service, *registrytest.Registry, *countingTransport) {
	t.Helper()
	s, _, _ := newTestService()
	reg := registrytest.New(t)
	transport := &countingTransport{next: reg.Transport()}
	s.ImageCheck = &registryread.Checker{Transport: transport}
	return s, reg, transport
}

// checkGQL runs isDeployableImage and returns its result object.
func checkGQL(t *testing.T, s *Service, ctx context.Context, image, credentialID string) (map[string]any, []string) {
	t.Helper()
	input := fmt.Sprintf(`{imageUrl: %q}`, image)
	if credentialID != "" {
		input = fmt.Sprintf(`{imageUrl: %q, registryCredentialId: %q}`, image, credentialID)
	}
	res := graphql.Do(graphql.Params{Schema: testSchema(s), Context: ctx,
		RequestString: `{ isDeployableImage(externalImageReference: ` + input + `) { deployable failureCode failureReason } }`})
	var errs []string
	for _, e := range res.Errors {
		errs = append(errs, e.Message)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return res.Data.(map[string]any)["isDeployableImage"].(map[string]any), nil
}

func wantAnswer(t *testing.T, got map[string]any, errs []string, deployable bool, code string) {
	t.Helper()
	if len(errs) > 0 {
		t.Fatalf("isDeployableImage errors: %v", errs)
	}
	if got["deployable"] != deployable {
		t.Fatalf("deployable = %v, want %v (%v)", got["deployable"], deployable, got)
	}
	if code == "" {
		if got["failureCode"] != nil {
			t.Fatalf("failureCode = %v, want null (%v)", got["failureCode"], got)
		}
	} else if got["failureCode"] != code {
		t.Fatalf("failureCode = %v, want %s (%v)", got["failureCode"], code, got)
	}
	if deployable && got["failureReason"] != nil {
		t.Fatalf("a deployable image carries failureReason %v", got["failureReason"])
	}
	if !deployable && got["failureReason"] == nil {
		t.Fatalf("an undeployable image carries no failureReason: %v", got)
	}
}

func TestIsDeployableImageAnswersEachCodeOverGraphQL(t *testing.T) {
	s, reg, _ := imageCheckService(t)
	ctx := context.Background()
	reg.Push("acme/app:1", registryread.DefaultPlatform)
	reg.Push("acme/arm:1", v1.Platform{OS: "linux", Architecture: "arm64"})
	reg.Push("private/app:1", registryread.DefaultPlatform)

	got, errs := checkGQL(t, s, ctx, registrytest.Ref("acme/app:1"), "")
	wantAnswer(t, got, errs, true, "")
	got, errs = checkGQL(t, s, ctx, registrytest.Ref("acme/app:2"), "")
	wantAnswer(t, got, errs, false, registryread.FailureDoesNotExist)
	got, errs = checkGQL(t, s, ctx, registrytest.Ref("acme/arm:1"), "")
	wantAnswer(t, got, errs, false, registryread.FailureUnsupportedPlatform)
	got, errs = checkGQL(t, s, ctx, "Not An Image", "")
	wantAnswer(t, got, errs, false, registryread.FailureInvalidInputURL)

	right, err := s.Create(ctx, CreateRequest{Host: registrytest.Host, Username: registrytest.Username, Secret: registrytest.Password})
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := s.Create(ctx, CreateRequest{Name: "wrong", Host: registrytest.Host, Username: registrytest.Username, Secret: "not-the-token"})
	if err != nil {
		t.Fatal(err)
	}
	got, errs = checkGQL(t, s, ctx, registrytest.Ref("private/app:1"), "")
	wantAnswer(t, got, errs, false, registryread.FailureDoesNotExist)
	got, errs = checkGQL(t, s, ctx, registrytest.Ref("private/app:1"), right.ID)
	wantAnswer(t, got, errs, true, "")
	got, errs = checkGQL(t, s, ctx, registrytest.Ref("private/app:1"), wrong.ID)
	wantAnswer(t, got, errs, false, registryread.FailureInvalidRegistryCredential)

	s.ImageCheck.MaxBytes = 1
	got, errs = checkGQL(t, s, ctx, registrytest.Ref("acme/app:1"), "")
	wantAnswer(t, got, errs, false, registryread.FailureImageTooLarge)
}

// What create refuses never reaches a registry: a loopback, a metadata
// address or a malformed reference answers invalid_input_url without a dial.
func TestIsDeployableImageRefusesPrivateHostsWithoutADial(t *testing.T) {
	s, _, transport := imageCheckService(t)
	for _, image := range []string{"127.0.0.1:5000/acme/app:1", "169.254.169.254/latest/meta-data", "localhost/acme/app", "[::1]:5000/acme/app", "UPPER/case"} {
		got, errs := checkGQL(t, s, context.Background(), image, "")
		wantAnswer(t, got, errs, false, registryread.FailureInvalidInputURL)
	}
	if n := transport.n.Load(); n != 0 {
		t.Fatalf("refused references made %d registry requests", n)
	}
}

// A credential pulls from its own registry only, so another registry's image
// with it is a credential the deploy would never use.
func TestIsDeployableImageRefusesACredentialForAnotherRegistry(t *testing.T) {
	s, reg, transport := imageCheckService(t)
	reg.Push("acme/app:1", registryread.DefaultPlatform)
	ctx := context.Background()
	ghcr, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "fixture-ghcr-token"})
	if err != nil {
		t.Fatal(err)
	}
	got, errs := checkGQL(t, s, ctx, registrytest.Ref("acme/app:1"), ghcr.ID)
	wantAnswer(t, got, errs, false, registryread.FailureInvalidRegistryCredential)
	if !strings.Contains(fmt.Sprint(got["failureReason"]), "ghcr.io") {
		t.Errorf("failureReason %v does not name the credential's registry", got["failureReason"])
	}
	if n := transport.n.Load(); n != 0 {
		t.Fatalf("a mismatched credential made %d registry requests", n)
	}
}

// A credential the caller cannot read answers exactly what
// registryCredential(id) answers, never a verdict, so an id cannot be probed.
func TestIsDeployableImageAnswersAnUnreadableCredentialAsRegistryCredentialDoes(t *testing.T) {
	s, reg, transport := imageCheckService(t)
	reg.Push("private/app:1", registryread.DefaultPlatform)
	s.Workspace = credentialWorkspaces{}
	s.Authz = credentialRoles{}
	admin := core.WithIdentity(context.Background(), core.Identity{Subject: "admin", Method: "session"})
	cred, err := s.Create(admin, CreateRequest{OwnerID: "tea-b", Host: registrytest.Host, Username: registrytest.Username, Secret: registrytest.Password})
	if err != nil {
		t.Fatal(err)
	}
	// From here the caller belongs to tea-a only.
	s.Workspace = onlyWorkspace{"tea-a"}
	for _, id := range []string{cred.ID, "rgc-missing"} {
		res := graphql.Do(graphql.Params{Schema: testSchema(s), Context: admin,
			RequestString: `{ registryCredential(id: "` + id + `") { id } }`})
		if len(res.Errors) == 0 {
			t.Fatalf("registryCredential(%s) answered a caller outside its workspace", id)
		}
		_, errs := checkGQL(t, s, admin, registrytest.Ref("private/app:1"), id)
		if len(errs) != 1 || errs[0] != res.Errors[0].Message {
			t.Fatalf("isDeployableImage with %s = %v; want registryCredential's error %q", id, errs, res.Errors[0].Message)
		}
	}
	if n := transport.n.Load(); n != 0 {
		t.Fatalf("an unreadable credential made %d registry requests", n)
	}
}

type onlyWorkspace struct{ id string }

func (o onlyWorkspace) Tenant(context.Context, core.Identity) (string, bool) { return o.id, true }
func (o onlyWorkspace) IsMember(_ context.Context, _ core.Identity, workspace string) (bool, error) {
	return workspace == o.id, nil
}

func TestIsDeployableImageBoundsAWorkspacesConcurrentChecks(t *testing.T) {
	s, reg, _ := imageCheckService(t)
	reg.Push("acme/app:1", registryread.DefaultPlatform)
	s.ImageCheck.PerWorkspace = 1
	release, ok := s.ImageCheck.Acquire(core.DefaultTenant)
	if !ok {
		t.Fatal("acquire")
	}
	_, err := s.IsDeployableImage(context.Background(), registrytest.Ref("acme/app:1"), "")
	var coded *core.CodedError
	if !errors.Is(err, core.ErrTooManyRequests) || !errors.As(err, &coded) || coded.Code != CodeImageCheckLimit {
		t.Fatalf("a check past the workspace's limit = %v, want 429 %s", err, CodeImageCheckLimit)
	}
	release()
	if v, err := s.IsDeployableImage(context.Background(), registrytest.Ref("acme/app:1"), ""); err != nil || !v.Deployable {
		t.Fatalf("after the slot freed: %+v, %v", v, err)
	}
}

// The verdict never carries the credential's secret.
func TestIsDeployableImageNeverEchoesTheSecret(t *testing.T) {
	s, reg, _ := imageCheckService(t)
	reg.Push("private/app:1", registryread.DefaultPlatform)
	ctx := context.Background()
	wrong, err := s.Create(ctx, CreateRequest{Host: registrytest.Host, Username: registrytest.Username, Secret: "fixture-wrong-secret"})
	if err != nil {
		t.Fatal(err)
	}
	got, errs := checkGQL(t, s, ctx, registrytest.Ref("private/app:1"), wrong.ID)
	wantAnswer(t, got, errs, false, registryread.FailureInvalidRegistryCredential)
	if b, _ := json.Marshal(got); strings.Contains(string(b), "fixture-wrong-secret") {
		t.Fatalf("the answer carries the secret: %s", b)
	}
}
