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
	"fmt"
	"regexp"
	"strings"
)

// Google Cloud Workload Identity Federation (docs/ADR116-managed-oidc.md
// D11, w2/m479). render.com/docs/oidc: GCP_WORKLOAD_IDENTITY_PROVIDER is
// "The resource name of the provider you created (starts with projects/).
// You can also paste the provider's full URL (starting with
// https://iam.googleapis.com/)", and GCP_SERVICE_ACCOUNT_EMAIL, optional, is
// "the email address of the service account to impersonate". Its
// troubleshooting: "Use your project number, not your project ID", and the
// provider's audience must be the default one.

const (
	// ManagedOIDCGCPName is the GCP provider's name in ManagedOIDCProviders.
	ManagedOIDCGCPName = "gcp"
	// GCPWorkloadIdentityProviderEnv is the GCP provider's trigger.
	GCPWorkloadIdentityProviderEnv = "GCP_WORKLOAD_IDENTITY_PROVIDER"
	// GCPServiceAccountEmailEnv names the service account the credential
	// configuration impersonates; unset, the service acts as its own
	// federated principal.
	GCPServiceAccountEmailEnv = "GCP_SERVICE_ACCOUNT_EMAIL"
)

// ManagedOIDCGCP is one release's Google Cloud inputs, read by the operator
// from the release's environment and turned into the credential
// configuration by bex-api.
type ManagedOIDCGCP struct {
	// WorkloadIdentityProvider is the workload identity pool provider's
	// resource name, projects/<project number>/locations/<location>/
	// workloadIdentityPools/<pool>/providers/<provider>, whichever of
	// Render's two spellings the service set.
	// +optional
	WorkloadIdentityProvider string `json:"workloadIdentityProvider,omitempty"`
	// ServiceAccountEmail is the service account to impersonate; empty for
	// direct access.
	// +optional
	ServiceAccountEmail string `json:"serviceAccountEmail,omitempty"`
	// Refused is why the variables cannot be used, as the deploy log says
	// it. The release then mounts no Google Cloud credential, so nothing is
	// minted for an audience Google would refuse.
	// +optional
	Refused string `json:"refused,omitempty"`
}

// Audience is the token's aud claim: the provider's default audience,
// "https://iam.googleapis.com/" and its resource name (Google: "Tokens
// contain an aud claim with the following value:
// https://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/
// workloadIdentityPools/POOL_ID/providers/WORKLOAD_PROVIDER_ID").
func (g ManagedOIDCGCP) Audience() string {
	return "https://iam.googleapis.com/" + g.WorkloadIdentityProvider
}

// The prefixes a provider value may carry before projects/: Render's full
// URL, and the full resource name a credential configuration's audience
// spells (what gcloud writes, so a value copied from one works).
var gcpProviderPrefixes = []string{"https://iam.googleapis.com/", "//iam.googleapis.com/"}

var (
	// gcpProjectNumber is a project number.
	gcpProjectNumber = regexp.MustCompile(`^[0-9]+$`)
	// gcpProjectID is a project id: "6 to 30 lowercase letters, digits, or
	// hyphens. It must start with a letter, and cannot have a trailing
	// hyphen."
	gcpProjectID = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	// gcpSegment is a location, pool or provider id.
	gcpSegment = regexp.MustCompile(`^[a-z0-9-]+$`)
	// gcpServiceAccountEmail is an email address whose every character is
	// one a service account's can carry, so the impersonation URL built
	// from it is the account's.
	gcpServiceAccountEmail = regexp.MustCompile(`^[A-Za-z0-9._+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)
)

// The deploy log's explanations for variables that cannot be used.
const (
	gcpNotMounted          = ", so no Google Cloud credential configuration is mounted"
	gcpProviderMalformed   = GCPWorkloadIdentityProviderEnv + " is not a workload identity pool provider's resource name (projects/<project number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>)" + gcpNotMounted
	gcpEmailMalformed      = GCPServiceAccountEmailEnv + " is not a service account's email address" + gcpNotMounted
	gcpProjectIDRefusedFmt = GCPWorkloadIdentityProviderEnv + " names project %q by its ID: Google Cloud expects the project number (IAM & Admin > Settings) in the provider's audience" + gcpNotMounted
)

// NewManagedOIDCGCP reads a release's GCP_WORKLOAD_IDENTITY_PROVIDER
// (provider, set and not blank) and GCP_SERVICE_ACCOUNT_EMAIL (email, blank
// when unset). Either of Render's spellings gives the same resource name. A
// project id where the project number belongs, or anything else that is not
// a provider's resource name or a service account's email, is Refused by
// name: Google refuses a token for any audience but the default one.
func NewManagedOIDCGCP(provider, email string) ManagedOIDCGCP {
	name, refused := parseGCPProvider(strings.TrimSpace(provider))
	if refused != "" {
		return ManagedOIDCGCP{Refused: refused}
	}
	email = strings.TrimSpace(email)
	if email != "" && !gcpServiceAccountEmail.MatchString(email) {
		return ManagedOIDCGCP{Refused: gcpEmailMalformed}
	}
	return ManagedOIDCGCP{WorkloadIdentityProvider: name, ServiceAccountEmail: email}
}

// parseGCPProvider is v's resource name, or why v is not one.
func parseGCPProvider(v string) (name, refused string) {
	for _, prefix := range gcpProviderPrefixes {
		if rest, ok := strings.CutPrefix(v, prefix); ok {
			v = rest
			break
		}
	}
	parts := strings.Split(v, "/")
	if len(parts) != 8 || parts[0] != "projects" || parts[2] != "locations" || parts[4] != "workloadIdentityPools" || parts[6] != "providers" {
		return "", gcpProviderMalformed
	}
	for _, i := range []int{3, 5, 7} {
		if !gcpSegment.MatchString(parts[i]) {
			return "", gcpProviderMalformed
		}
	}
	switch project := parts[1]; {
	case gcpProjectNumber.MatchString(project):
		return v, ""
	case gcpProjectID.MatchString(project):
		return "", fmt.Sprintf(gcpProjectIDRefusedFmt, project)
	default:
		return "", gcpProviderMalformed
	}
}
