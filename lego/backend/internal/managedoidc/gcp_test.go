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

package managedoidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2/google"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	gcpProvider = "projects/123456789012/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider"
	gcpAccount  = "svc@my-project.iam.gserviceaccount.com"
	cloudScope  = "https://www.googleapis.com/auth/cloud-platform"
)

func gcpTokenPath(t *testing.T) string {
	t.Helper()
	p, _ := appv1alpha1.ManagedOIDCProviderNamed(appv1alpha1.ManagedOIDCGCPName)
	return p.Path()
}

// TestGCPCredentialConfigurationIsWhatGcloudWrites (w2/m479 t002): the file
// GOOGLE_APPLICATION_CREDENTIALS names is byte for byte what gcloud writes for
// the same provider, service account and token file. The golden files are
// gcloud 593.0.0's (Google Cloud SDK, core 2026.11.06):
//
//	gcloud iam workload-identity-pools create-cred-config \
//	  projects/123456789012/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider \
//	  [--service-account=svc@my-project.iam.gserviceaccount.com] \
//	  --credential-source-file=/var/lib/render/oidc/gcp-token \
//	  --credential-source-type=text --output-file=testdata/gcloud-{direct,impersonate}.json
//
// Either of Render's spellings of GCP_WORKLOAD_IDENTITY_PROVIDER gives the
// same file, and the impersonation URL appears only with
// GCP_SERVICE_ACCOUNT_EMAIL set.
func TestGCPCredentialConfigurationIsWhatGcloudWrites(t *testing.T) {
	for golden, email := range map[string]string{"gcloud-direct.json": "", "gcloud-impersonate.json": gcpAccount} {
		want, err := os.ReadFile(filepath.Join("testdata", golden))
		if err != nil {
			t.Fatal(err)
		}
		for _, spelling := range []string{gcpProvider, "https://iam.googleapis.com/" + gcpProvider, "  " + gcpProvider + "\n"} {
			inputs := appv1alpha1.NewManagedOIDCGCP(spelling, email)
			if inputs.Refused != "" {
				t.Fatalf("%q refused: %s", spelling, inputs.Refused)
			}
			got := gcpCredentialConfiguration(inputs, gcpTokenPath(t))
			if string(got) != string(want) {
				t.Errorf("GCP_WORKLOAD_IDENTITY_PROVIDER=%q GCP_SERVICE_ACCOUNT_EMAIL=%q:\n%s\nwant gcloud's %s:\n%s", spelling, email, got, golden, want)
			}
		}
		impersonates := strings.Contains(string(want), "service_account_impersonation_url")
		if impersonates != (email != "") {
			t.Errorf("%s: impersonation %v with GCP_SERVICE_ACCOUNT_EMAIL=%q", golden, impersonates, email)
		}
	}
}

// fakeGoogle is Google's STS and IAM Credentials API as Google's Go library
// calls them for an external_account configuration.
type fakeGoogle struct {
	mu            sync.Mutex
	subjectTokens []string // the subject_token of each STS exchange
	stsForms      []map[string]string
	impersonated  []string // the path and bearer token of each generateAccessToken call
}

func (g *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/v1/token":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		form := map[string]string{}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		g.stsForms = append(g.stsForms, form)
		g.subjectTokens = append(g.subjectTokens, form["subject_token"])
		// expires_in 1 makes the library exchange again on the next call.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "federated-" + form["subject_token"], "issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
			"token_type": "Bearer", "expires_in": 1,
		})
	case strings.HasSuffix(r.URL.Path, ":generateAccessToken"):
		g.impersonated = append(g.impersonated, r.URL.Path+" "+r.Header.Get("Authorization"))
		// Expiring at once, so the next call exchanges again too.
		_ = json.NewEncoder(w).Encode(map[string]any{"accessToken": "impersonated", "expireTime": time.Now().Add(time.Second).UTC().Format(time.RFC3339)})
	default:
		http.NotFound(w, r)
	}
}

// TestGoogleAuthExchangesTheMountedTokenForCloudPlatform (w2/m479 t002):
// golang.org/x/oauth2/google, the library behind Google's Go client
// libraries' Application Default Credentials, constructs credentials from
// the generated file and, asked for the cloud-platform scope as Render's
// troubleshooting says, exchanges the token in credential_source.file at
// Google's STS for the provider's audience, then impersonates the service
// account when the file names one. Each exchange rereads the file, so the
// refreshed token is the one sent. Only the endpoints are pointed at a
// stand-in for Google.
func TestGoogleAuthExchangesTheMountedTokenForCloudPlatform(t *testing.T) {
	for name, email := range map[string]string{"direct access": "", "impersonation": gcpAccount} {
		t.Run(name, func(t *testing.T) {
			goog := &fakeGoogle{}
			srv := httptest.NewServer(goog)
			t.Cleanup(srv.Close)
			tokenFile := filepath.Join(t.TempDir(), "gcp-token")
			if err := os.WriteFile(tokenFile, []byte("first-token"), 0o600); err != nil {
				t.Fatal(err)
			}

			var cfg map[string]any
			if err := json.Unmarshal(gcpCredentialConfiguration(appv1alpha1.ManagedOIDCGCP{WorkloadIdentityProvider: gcpProvider, ServiceAccountEmail: email}, gcpTokenPath(t)), &cfg); err != nil {
				t.Fatal(err)
			}
			cfg["credential_source"].(map[string]any)["file"] = tokenFile
			cfg["token_url"] = srv.URL + "/v1/token"
			if u, ok := cfg["service_account_impersonation_url"].(string); ok {
				cfg["service_account_impersonation_url"] = strings.Replace(u, "https://iamcredentials.googleapis.com", srv.URL, 1)
			}
			raw, _ := json.Marshal(cfg)

			creds, err := google.CredentialsFromJSONWithType(context.Background(), raw, google.ExternalAccount, cloudScope)
			if err != nil {
				t.Fatalf("google.CredentialsFromJSONWithType: %v", err)
			}
			if _, err := creds.TokenSource.Token(); err != nil {
				t.Fatalf("first token: %v", err)
			}
			if err := os.WriteFile(tokenFile, []byte("refreshed-token\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			tok, err := creds.TokenSource.Token()
			if err != nil {
				t.Fatalf("second token: %v", err)
			}

			goog.mu.Lock()
			defer goog.mu.Unlock()
			if len(goog.subjectTokens) != 2 || goog.subjectTokens[0] != "first-token" || goog.subjectTokens[1] != "refreshed-token" {
				t.Fatalf("subject tokens sent = %q, want the file's token at each exchange, refreshed the second time", goog.subjectTokens)
			}
			form := goog.stsForms[0]
			for k, want := range map[string]string{
				"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
				"audience":             "//iam.googleapis.com/" + gcpProvider,
				"subject_token_type":   "urn:ietf:params:oauth:token-type:jwt",
				"requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
			} {
				if form[k] != want {
					t.Errorf("STS %s = %q, want %q", k, form[k], want)
				}
			}
			if email == "" {
				if form["scope"] != cloudScope || len(goog.impersonated) != 0 || tok.AccessToken != "federated-refreshed-token" {
					t.Errorf("direct access: STS scope %q, impersonation calls %q, token %q; want cloud-platform, none, the federated token", form["scope"], goog.impersonated, tok.AccessToken)
				}
				return
			}
			want := "/v1/projects/-/serviceAccounts/" + gcpAccount + ":generateAccessToken Bearer federated-first-token"
			if len(goog.impersonated) == 0 || goog.impersonated[0] != want || tok.AccessToken != "impersonated" {
				t.Errorf("impersonation calls %q, token %q; want %q first and the service account's token", goog.impersonated, tok.AccessToken, want)
			}
		})
	}
}
