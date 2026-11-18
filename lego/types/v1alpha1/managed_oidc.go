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

import "slices"

// Managed OIDC delivery (docs/ADR116-managed-oidc.md D9, w2/m477): a service
// whose environment sets a provider's trigger variable gets that provider's
// identity token as a read-only file, as on Render. Both sides of the App CR
// boundary use these names: bex-api projects the plan label, mints the token
// Secret and reads the status; the operator mounts the Secret and records the
// status.

// LabelManagedOIDC carries the workspace plan's Managed OIDC allowance,
// projected by bex-api (ManagedOIDCEnabled, or absent below Pro and when no
// issuer is configured). The operator reads it once per release, when it
// decides that release's mounts (ManagedOIDCStatus.PlanAllowed).
const LabelManagedOIDC = "app.bex.co/managed-oidc"

// ManagedOIDCEnabled is LabelManagedOIDC's one value.
const ManagedOIDCEnabled = "enabled"

// ManagedOIDCMountPath is the read-only directory the token files appear in,
// Render's own ("a read-only bind mount between /var/lib/render/oidc/ inside
// the container" and memory-backed storage on the host; render.com/blog/how-we-built-render-oidc).
const ManagedOIDCMountPath = "/var/lib/render/oidc"

// ManagedOIDCSecretName is the Secret bex-api keeps an App's tokens in, one
// key per provider (ManagedOIDCProvider.File). It is not a release
// configuration source: pods reference it by this mutable name, so a refresh
// reaches every running instance.
func ManagedOIDCSecretName(app string) string { return app + "-oidc" }

// ManagedOIDCProvider is one provider Render's Managed OIDC delivers a token
// for, keyed by the variable that turns it on.
type ManagedOIDCProvider struct {
	// Name is the provider's id in ManagedOIDCStatus.Providers.
	Name string
	// Trigger is the variable a service sets to turn the provider on.
	Trigger string
	// FileEnv is the variable the platform sets to the token file's path.
	FileEnv string
	// File is the token's Secret key and its file name under the mount.
	File string
	// Audience is the token's aud claim; empty when each release fixes its
	// own (GCP: ManagedOIDCGCP.Audience).
	Audience string
	// Config, when set, is the Secret key and file name of the credential
	// configuration bex-api writes beside the token, which FileEnv names in
	// place of the token: Google's libraries read the configuration, and it
	// names the token file.
	Config string
}

// Path is the token file's path in the container.
func (p ManagedOIDCProvider) Path() string { return ManagedOIDCMountPath + "/" + p.File }

// FileEnvPath is FileEnv's value: the credential configuration's path when
// the provider has one, else the token's.
func (p ManagedOIDCProvider) FileEnvPath() string {
	if p.Config != "" {
		return ManagedOIDCMountPath + "/" + p.Config
	}
	return p.Path()
}

// Files is the provider's Secret keys, each also its file name under the
// mount: the token, then the credential configuration when there is one.
func (p ManagedOIDCProvider) Files() []string {
	if p.Config != "" {
		return []string{p.File, p.Config}
	}
	return []string{p.File}
}

// ManagedOIDCProviders lists the providers in the order they are mounted.
// Each has its own audience, so a service with several gets one token per
// provider and a token one provider accepts is refused by the others. The
// trigger is the one variable render.com/docs/oidc says Render detects for
// the provider; its companions (the Anthropic organization, service account
// and workspace ids, the OpenAI service account id) are read by the SDK, not
// by the platform (docs/ADR116-managed-oidc.md D10). Render names no file in
// its directory, so the file names are bex's.
//
//   - AWS: "add the AWS_ROLE_ARN environment variable"; audience
//     sts.amazonaws.com; Render "automatically sets an additional
//     environment variable named AWS_WEB_IDENTITY_TOKEN_FILE".
//   - GCP: "Render detects GCP_WORKLOAD_IDENTITY_PROVIDER and writes a
//     credential configuration file for your service's OIDC token. Render
//     then sets an additional environment variable,
//     GOOGLE_APPLICATION_CREDENTIALS, to the path of the new file"; the
//     audience is the provider's default audience (ManagedOIDCGCP).
//   - Anthropic: "Render detects the ANTHROPIC_FEDERATION_RULE_ID
//     environment variable and automatically sets an additional environment
//     variable named ANTHROPIC_IDENTITY_TOKEN_FILE"; expected audience
//     api.anthropic.com.
//   - OpenAI: "Render detects the OPENAI_IDENTITY_PROVIDER_ID environment
//     variable and automatically sets an additional environment variable
//     named OPENAI_IDENTITY_TOKEN_FILE"; audience api.openai.com.
//
// The order is Render's ("AWS, GCP, Anthropic, OpenAI"). The entries are
// literals: scripts/oidc-audience-verify.test.sh reads them.
var ManagedOIDCProviders = []ManagedOIDCProvider{
	{Name: "aws", Trigger: "AWS_ROLE_ARN", FileEnv: "AWS_WEB_IDENTITY_TOKEN_FILE", File: "aws-token", Audience: "sts.amazonaws.com"},
	{Name: "gcp", Trigger: "GCP_WORKLOAD_IDENTITY_PROVIDER", FileEnv: "GOOGLE_APPLICATION_CREDENTIALS", File: "gcp-token", Config: "gcp-credentials.json"},
	{Name: "anthropic", Trigger: "ANTHROPIC_FEDERATION_RULE_ID", FileEnv: "ANTHROPIC_IDENTITY_TOKEN_FILE", File: "anthropic-token", Audience: "api.anthropic.com"},
	{Name: "openai", Trigger: "OPENAI_IDENTITY_PROVIDER_ID", FileEnv: "OPENAI_IDENTITY_TOKEN_FILE", File: "openai-token", Audience: "api.openai.com"},
}

// ManagedOIDCProviderNamed returns the provider called name.
func ManagedOIDCProviderNamed(name string) (ManagedOIDCProvider, bool) {
	i := slices.IndexFunc(ManagedOIDCProviders, func(p ManagedOIDCProvider) bool { return p.Name == name })
	if i < 0 {
		return ManagedOIDCProvider{}, false
	}
	return ManagedOIDCProviders[i], true
}

// ManagedOIDCProvidersNamed is the providers called names, in names' order;
// a name no provider has is skipped.
func ManagedOIDCProvidersNamed(names []string) []ManagedOIDCProvider {
	var out []ManagedOIDCProvider
	for _, name := range names {
		if p, ok := ManagedOIDCProviderNamed(name); ok {
			out = append(out, p)
		}
	}
	return out
}

// ManagedOIDCStatus is the Managed OIDC decision of one release, recorded by
// the operator when it first projects the release and kept for its life. A
// release that was already serving when the operator first saw it mounts
// nothing.
type ManagedOIDCStatus struct {
	// Generation is the release generation the decision belongs to.
	Generation int64 `json:"generation"`
	// Providers are the providers whose trigger the release's environment
	// sets, in ManagedOIDCProviders order.
	// +optional
	Providers []string `json:"providers,omitempty"`
	// PlanAllowed records that the App carried LabelManagedOIDC when the
	// release was decided, so a plan change takes effect at the next deploy.
	// +optional
	PlanAllowed bool `json:"planAllowed,omitempty"`
	// GCP is the release's Google Cloud inputs, set when its environment sets
	// GCP_WORKLOAD_IDENTITY_PROVIDER: what the credential configuration and
	// the token's audience are made of, or why they cannot be (Refused, and
	// then Providers leaves gcp out).
	// +optional
	GCP *ManagedOIDCGCP `json:"gcp,omitempty"`
}

// Mounted is the providers whose token files the release's pods read: none
// when the plan did not allow it.
func (s *ManagedOIDCStatus) Mounted() []string {
	if s == nil || !s.PlanAllowed {
		return nil
	}
	return s.Providers
}

// Refused is the providers the release asked for that its workspace plan did
// not include, which the deploy log names.
func (s *ManagedOIDCStatus) Refused() []string {
	if s == nil || s.PlanAllowed {
		return nil
	}
	return s.Providers
}
