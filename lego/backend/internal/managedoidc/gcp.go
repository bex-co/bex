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
	"encoding/json"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// gcpCredentialConfig is Google's external_account credential configuration
// for a token read from a file, field for field and in the order
// `gcloud iam workload-identity-pools create-cred-config
// --credential-source-file=<file> --credential-source-type=text` writes it
// (testdata/gcloud-*.json, docs/ADR116-managed-oidc.md D11). gcloud writes
// token_info_url only when the service acts as its own federated principal,
// and service_account_impersonation_url only when it impersonates one.
type gcpCredentialConfig struct {
	UniverseDomain                 string              `json:"universe_domain"`
	Type                           string              `json:"type"`
	Audience                       string              `json:"audience"`
	SubjectTokenType               string              `json:"subject_token_type"`
	TokenURL                       string              `json:"token_url"`
	CredentialSource               gcpCredentialSource `json:"credential_source"`
	ServiceAccountImpersonationURL string              `json:"service_account_impersonation_url,omitempty"`
	TokenInfoURL                   string              `json:"token_info_url,omitempty"`
}

type gcpCredentialSource struct {
	File   string    `json:"file"`
	Format gcpFormat `json:"format"`
}

type gcpFormat struct {
	Type string `json:"type"`
}

// gcpCredentialConfiguration is the file GOOGLE_APPLICATION_CREDENTIALS
// names for a release with inputs gcp, reading its token from tokenPath:
// byte for byte what gcloud writes for the same provider, service account
// and token file (two-space indent, no trailing newline).
func gcpCredentialConfiguration(gcp appv1alpha1.ManagedOIDCGCP, tokenPath string) []byte {
	cfg := gcpCredentialConfig{
		UniverseDomain:   "googleapis.com",
		Type:             "external_account",
		Audience:         "//iam.googleapis.com/" + gcp.WorkloadIdentityProvider,
		SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:         "https://sts.googleapis.com/v1/token",
		CredentialSource: gcpCredentialSource{File: tokenPath, Format: gcpFormat{Type: "text"}},
	}
	if gcp.ServiceAccountEmail != "" {
		cfg.ServiceAccountImpersonationURL = "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + gcp.ServiceAccountEmail + ":generateAccessToken"
	} else {
		cfg.TokenInfoURL = "https://sts.googleapis.com/v1/introspect"
	}
	out, _ := json.MarshalIndent(cfg, "", "  ") // a struct of strings always encodes
	return out
}
