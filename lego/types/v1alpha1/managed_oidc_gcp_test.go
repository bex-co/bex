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

package v1alpha1

import (
	"strings"
	"testing"
)

const testGCPProvider = "projects/123456789012/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider"

// TestNewManagedOIDCGCPTakesRendersTwoSpellings (w2/m479 t001): the
// resource name Render documents ("starts with projects/") and the full URL
// ("starting with https://iam.googleapis.com/") give the same resource name
// and so the same audience, Google's default one for the provider.
func TestNewManagedOIDCGCPTakesRendersTwoSpellings(t *testing.T) {
	for _, v := range []string{
		testGCPProvider,
		"https://iam.googleapis.com/" + testGCPProvider,
		"//iam.googleapis.com/" + testGCPProvider, // a credential configuration's audience
		" " + testGCPProvider + "\n",
	} {
		got := NewManagedOIDCGCP(v, "")
		if got != (ManagedOIDCGCP{WorkloadIdentityProvider: testGCPProvider}) {
			t.Errorf("NewManagedOIDCGCP(%q) = %+v, want the resource name", v, got)
		}
		if want := "https://iam.googleapis.com/" + testGCPProvider; got.Audience() != want {
			t.Errorf("audience of %q = %q, want the default audience %q", v, got.Audience(), want)
		}
	}
	got := NewManagedOIDCGCP(testGCPProvider, " svc@my-project.iam.gserviceaccount.com ")
	if got != (ManagedOIDCGCP{WorkloadIdentityProvider: testGCPProvider, ServiceAccountEmail: "svc@my-project.iam.gserviceaccount.com"}) {
		t.Errorf("with a service account: %+v", got)
	}
}

// TestNewManagedOIDCGCPRefusesWhatGoogleWouldRefuseByName: Render's
// troubleshooting names the project id as the usual mistake ("Use your
// project number, not your project ID"); bex names it in the deploy log
// instead of minting a token for an audience Google refuses. Anything else
// that is not a provider's resource name, or a service account email that
// would not name an account in the impersonation URL, is refused too.
func TestNewManagedOIDCGCPRefusesWhatGoogleWouldRefuseByName(t *testing.T) {
	const notMounted = ", so no Google Cloud credential configuration is mounted"
	projectID := NewManagedOIDCGCP("https://iam.googleapis.com/projects/my-project/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider", "")
	if want := `GCP_WORKLOAD_IDENTITY_PROVIDER names project "my-project" by its ID: Google Cloud expects the project number (IAM & Admin > Settings) in the provider's audience` + notMounted; projectID.Refused != want {
		t.Errorf("a project id: Refused = %q\nwant %q", projectID.Refused, want)
	}
	if projectID.WorkloadIdentityProvider != "" {
		t.Errorf("a refused value keeps a resource name to mint for: %+v", projectID)
	}
	for _, v := range []string{
		"bex-provider",
		"projects/123456789012",
		"projects/123456789012/locations/global/workloadIdentityPools/bex-pool",
		"projects/123456789012/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider/",
		"projects/123456789012/locations/global/workloadIdentityPools/bex-pool/providers/",
		"projects/123456789012/locations/global/workloadIdentityPools/bex-pool/providers/Bex Provider",
		"projects/123456789012/locations/global/workloadIdentityPools/bex-pool/provider/bex-provider",
		"http://iam.googleapis.com/" + testGCPProvider,
		"https://iam.googleapis.com/" + testGCPProvider + "?x=1",
		"https://sts.googleapis.com/" + testGCPProvider,
		"projects/My_Project!/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider",
		"projects/-/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider",
	} {
		got := NewManagedOIDCGCP(v, "")
		if !strings.HasPrefix(got.Refused, "GCP_WORKLOAD_IDENTITY_PROVIDER is not a workload identity pool provider's resource name (projects/<project number>/") || !strings.HasSuffix(got.Refused, notMounted) {
			t.Errorf("NewManagedOIDCGCP(%q).Refused = %q, want the resource-name refusal", v, got.Refused)
		}
		if got.WorkloadIdentityProvider != "" {
			t.Errorf("NewManagedOIDCGCP(%q) keeps %q", v, got.WorkloadIdentityProvider)
		}
	}
	for _, email := range []string{"svc", "svc@", "@my-project.iam.gserviceaccount.com", "svc@localhost", "svc@my project.iam.gserviceaccount.com", "svc/../x@my-project.iam.gserviceaccount.com", "svc@my-project.iam.gserviceaccount.com?x", "a@b@c.com"} {
		got := NewManagedOIDCGCP(testGCPProvider, email)
		if want := "GCP_SERVICE_ACCOUNT_EMAIL is not a service account's email address" + notMounted; got.Refused != want || got.WorkloadIdentityProvider != "" {
			t.Errorf("GCP_SERVICE_ACCOUNT_EMAIL=%q: %+v, want refused with %q", email, got, want)
		}
	}
}
