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

package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appSpecIdentityClass is the exhaustive policy for deciding what an AppSpec
// mutation means. Artifact fields can require source access/a build. Release
// fields can require pre-deploy and a workload rollout. Operational fields are
// reconciled against the current release and must do neither.
type appSpecIdentityClass uint8

const (
	identityOperational appSpecIdentityClass = 0
	identityArtifact    appSpecIdentityClass = 1 << 0
	identityRelease     appSpecIdentityClass = 1 << 1
)

// appSpecIdentityClasses deliberately names every AppSpec field. The guard test
// in release_identity_test.go fails when the CRD contract gains a field without
// a deploy-semantics decision here.
var appSpecIdentityClasses = map[string]appSpecIdentityClass{
	"DisplayName":                identityOperational,
	"Type":                       identityRelease,
	"Schedule":                   identityOperational,
	"Command":                    identityRelease,
	"RunAt":                      identityOperational,
	"CancelRun":                  identityOperational,
	"PublishPath":                identityRelease,
	"Routes":                     identityOperational,
	"Headers":                    identityOperational,
	"Repo":                       identityArtifact | identityRelease,
	"Image":                      identityArtifact | identityRelease,
	"RootDir":                    identityArtifact | identityRelease,
	"BuildFilter":                identityOperational,
	"DockerfilePath":             identityArtifact | identityRelease,
	"DockerContext":              identityArtifact | identityRelease,
	"Branch":                     identityArtifact | identityRelease,
	"BuildCommit":                identityArtifact | identityRelease,
	"CloneSecret":                identityOperational,
	"ExternalRegistryPullSecret": identityArtifact | identityRelease,
	"RegistryCredentialID":       identityArtifact | identityRelease,
	"Runtime":                    identityArtifact | identityRelease,
	"BuildCommand":               identityArtifact | identityRelease,
	"StartCommand":               identityArtifact | identityRelease,
	"Builder":                    identityArtifact | identityRelease,
	"Replicas":                   identityOperational,
	"Port":                       identityRelease,
	"Env":                        identityArtifact | identityRelease,
	"EnvFromSecret":              identityArtifact | identityRelease,
	"EnvFromSecrets":             identityRelease,
	"FilesFromSecrets":           identityRelease,
	"HealthCheckPath":            identityRelease,
	"MaxShutdownDelaySeconds":    identityRelease,
	"AutoDeploy":                 identityOperational,
	"NotifyOnFail":               identityOperational,
	"NotificationsToSend":        identityOperational,
	"PreDeployCommand":           identityRelease,
	"IdleTTLSeconds":             identityOperational,
	"RestartedAt":                identityArtifact | identityRelease,
	"Suspended":                  identityOperational,
	"Autoscaling":                identityOperational,
	"Tier":                       identityRelease,
	"Host":                       identityOperational,
	"Expose":                     identityOperational,
	"Subdomain":                  identityOperational,
	"Hosts":                      identityOperational,
	"HostRedirects":              identityOperational,
	"SubdomainPolicy":            identityOperational,
	"IPAllowList":                identityOperational,
	"IPAllowListEntries":         identityOperational,
	"EnvironmentIPAllowList":     identityOperational,
	"MaintenanceMode":            identityOperational,
}

type appReleaseIdentity struct {
	artifact string
	release  string
}

// artifactIdentityInput contains only inputs that can change produced bytes or
// select a different prebuilt image. Credential values never enter the App spec
// and Secret names are represented only by the resulting one-way fingerprint.
type artifactIdentityInput struct {
	Repo                       string          `json:"repo,omitempty"`
	Image                      string          `json:"image,omitempty"`
	RootDir                    string          `json:"rootDir,omitempty"`
	DockerfilePath             string          `json:"dockerfilePath,omitempty"`
	DockerContext              string          `json:"dockerContext,omitempty"`
	Branch                     string          `json:"branch,omitempty"`
	BuildCommit                string          `json:"buildCommit,omitempty"`
	ExternalRegistryPullSecret string          `json:"externalRegistryPullSecret,omitempty"`
	RegistryCredentialID       *string         `json:"registryCredentialId,omitempty"`
	Runtime                    string          `json:"runtime,omitempty"`
	BuildCommand               string          `json:"buildCommand,omitempty"`
	StartCommand               string          `json:"startCommand,omitempty"`
	Builder                    string          `json:"builder,omitempty"`
	BuildEnv                   []corev1.EnvVar `json:"buildEnv,omitempty"`
	RuntimeEnvSecret           string          `json:"runtimeEnvSecret,omitempty"`
	RestartedAt                string          `json:"restartedAt,omitempty"`
}

type releaseIdentityInput struct {
	Artifact                   string               `json:"artifact"`
	Type                       string               `json:"type"`
	Command                    string               `json:"command,omitempty"`
	PublishPath                string               `json:"publishPath,omitempty"`
	ExternalRegistryPullSecret string               `json:"externalRegistryPullSecret,omitempty"`
	RegistryCredentialID       *string              `json:"registryCredentialId,omitempty"`
	StartCommand               string               `json:"startCommand,omitempty"`
	Port                       int32                `json:"port"`
	Env                        []appv1alpha1.EnvVar `json:"env,omitempty"`
	EnvFromSecret              string               `json:"envFromSecret,omitempty"`
	EnvFromSecrets             []string             `json:"envFromSecrets,omitempty"`
	FilesFromSecrets           []string             `json:"filesFromSecrets,omitempty"`
	HealthCheckPath            string               `json:"healthCheckPath,omitempty"`
	MaxShutdownDelaySeconds    *int32               `json:"maxShutdownDelaySeconds,omitempty"`
	PreDeployCommand           string               `json:"preDeployCommand,omitempty"`
	RestartedAt                string               `json:"restartedAt,omitempty"`
	Tier                       string               `json:"tier,omitempty"`
}

func desiredAppReleaseIdentity(spec appv1alpha1.AppSpec) appReleaseIdentity {
	builder := effectiveBuilder(spec)
	branch := spec.Branch
	if spec.Repo != "" && branch == "" {
		branch = appv1alpha1.DefaultBranch
	}

	artifactInput := artifactIdentityInput{
		Repo:                       spec.Repo,
		Image:                      spec.Image,
		RootDir:                    spec.RootDir,
		DockerfilePath:             spec.DockerfilePath,
		DockerContext:              spec.DockerContext,
		Branch:                     branch,
		BuildCommit:                spec.BuildCommit,
		ExternalRegistryPullSecret: spec.ExternalRegistryPullSecret,
		RegistryCredentialID:       spec.RegistryCredentialID,
		Runtime:                    spec.Runtime,
		BuildCommand:               spec.BuildCommand,
		Builder:                    builder,
		RestartedAt:                spec.RestartedAt,
	}
	// Native/buildpack builders bake the launch command into the image. Their
	// selected literal build env must also participate in the artifact identity;
	// buildEnv applies the same native-vs-BP_/BPE_ filter as the build plane.
	// Dockerfile/prebuilt services apply these only to the release pod below.
	if builder == build.BuilderNative || builder == build.BuilderBuildpack {
		artifactInput.StartCommand = spec.StartCommand
		artifactInput.BuildEnv = buildEnv(builder, spec.Env)
	}
	if builder == build.BuilderNative {
		artifactInput.RuntimeEnvSecret = spec.EnvFromSecret
	}

	artifact := identityFingerprint("artifact-v1", artifactInput)
	serviceType := spec.Type
	if serviceType == "" {
		serviceType = appv1alpha1.TypeWebService
	}
	port := spec.EffectivePort()
	healthCheckPath := spec.HealthCheckPath
	if healthCheckPath == "" {
		healthCheckPath = "/"
	}
	release := identityFingerprint("release-v1", releaseIdentityInput{
		Artifact:                   artifact,
		Type:                       serviceType,
		Command:                    spec.Command,
		PublishPath:                spec.PublishPath,
		ExternalRegistryPullSecret: spec.ExternalRegistryPullSecret,
		RegistryCredentialID:       spec.RegistryCredentialID,
		StartCommand:               spec.StartCommand,
		Port:                       port,
		Env:                        spec.Env,
		EnvFromSecret:              spec.EnvFromSecret,
		EnvFromSecrets:             spec.EnvFromSecrets,
		FilesFromSecrets:           spec.FilesFromSecrets,
		HealthCheckPath:            healthCheckPath,
		MaxShutdownDelaySeconds:    spec.MaxShutdownDelaySeconds,
		PreDeployCommand:           spec.PreDeployCommand,
		RestartedAt:                spec.RestartedAt,
		Tier:                       spec.Tier,
	})
	return appReleaseIdentity{artifact: artifact, release: release}
}

func identityFingerprint(version string, value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal %s identity: %v", version, err))
	}
	sum := sha256.Sum256(b)
	return version + ":" + hex.EncodeToString(sum[:])
}

type appReleaseDecision struct {
	desired         appReleaseIdentity
	artifactChanged bool
	releaseChanged  bool
	canceled        bool
}

// buildRunning reports whether a repo-backed source build for the pinned release
// generation is actively running (ADR060 §D1a). While true, prepareAppReleaseDecision
// pins the release identity so a newer push cannot abandon the running build:
// the build finishes and rolls out, and the newer spec waits in the implicit
// pending slot. Prebuilt-image, suspended, and direct-static-publish Apps have no
// build Job to protect and never pin.
func buildRunning(app *appv1alpha1.App) bool {
	if app.Spec.Repo == "" || app.Spec.Image != "" || app.Spec.Suspended {
		return false
	}
	if directStaticPublish(app) {
		return false
	}
	return app.Status.Phase == appv1alpha1.PhaseBuilding
}

// prepareAppReleaseDecision mutates only operator-owned status. Missing or
// changed fingerprints always request normal artifact/release reconciliation;
// every retained App has been normalized to the canonical status shape.
func prepareAppReleaseDecision(app *appv1alpha1.App) appReleaseDecision {
	desired := desiredAppReleaseIdentity(app.Spec)
	if generation, ok := canceledReleaseGeneration(app); ok && generation == requestedReleaseGeneration(app) {
		// Cancel deletes the deterministic build artifact, but reconciliation is
		// level-triggered. Keep the last successful generation active so this
		// pass cannot recreate the canceled build or falsely promote its release.
		app.Status.ReleaseGeneration = successfulReleaseGeneration(app)
		app.Status.PendingReleaseGeneration = 0
		return appReleaseDecision{desired: desired, canceled: true}
	}

	// ADR060 §D1a run-to-completion + latest-pending slot: while a source build is
	// actively running and the spec has since moved to a newer release, do NOT
	// advance. Pin the release identity to the running build so releaseBuildRevision
	// stays on its revision (EnsureBuild observes it, never starts a second Job) and
	// so the completing build records its artifact under its OWN fingerprint — not
	// the coalesced newer spec's, which would make the next generation falsely reuse
	// this image. The newer spec is the implicit pending slot, picked up on the next
	// reconcile once this build resolves (phase leaves Building) and the release
	// advances. This replaces ADR034's cancel-the-running-build newest-wins, which
	// livelocked under sustained pushes.
	if buildRunning(app) && app.Status.ReleaseFingerprint != desired.release {
		if pending := requestedReleaseGeneration(app); pending > app.Status.PendingReleaseGeneration {
			// A genuinely newer generation coalesced into the pending slot. Meter it
			// once per new pending generation — the tripwire that separates a
			// supersede (expected) from a user Cancel (canceled) in the SLIs.
			recordBuildOutcome(buildOutcomeSuperseded)
			app.Status.PendingReleaseGeneration = pending
		}
		pinned := appReleaseIdentity{
			artifact: app.Status.ReleaseArtifactFingerprint,
			release:  app.Status.ReleaseFingerprint,
		}
		return appReleaseDecision{
			desired:         pinned,
			artifactChanged: app.Status.ArtifactFingerprint != pinned.artifact,
			releaseChanged:  false,
		}
	}
	app.Status.PendingReleaseGeneration = 0

	artifactChanged := app.Status.ArtifactFingerprint != desired.artifact
	releaseChanged := app.Status.ReleaseFingerprint != desired.release
	if releaseChanged {
		app.Status.ReleaseFingerprint = desired.release
		// Pin the artifact fingerprint of the release being dispatched, so a push
		// that coalesces mid-build (above) can label the running build's resolved
		// artifact correctly (ADR060 §D1a).
		app.Status.ReleaseArtifactFingerprint = desired.artifact
		app.Status.ReleaseGeneration = requestedReleaseGeneration(app)
	}
	if app.Status.ReleaseGeneration == 0 {
		app.Status.ReleaseGeneration = app.Generation
	}
	return appReleaseDecision{
		desired:         desired,
		artifactChanged: artifactChanged,
		releaseChanged:  releaseChanged,
	}
}

func canceledReleaseGeneration(app *appv1alpha1.App) (int64, bool) {
	raw := app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration]
	generation, err := strconv.ParseInt(raw, 10, 64)
	return generation, err == nil && generation > 0
}

func successfulReleaseGeneration(app *appv1alpha1.App) int64 {
	if raw, ok := strings.CutPrefix(app.Status.ActiveRevision, "rev-"); ok {
		if generation, err := strconv.ParseInt(raw, 10, 64); err == nil && generation > 0 {
			return generation
		}
	}
	if app.Status.Image != "" && app.Status.ObservedGeneration > 0 {
		return app.Status.ObservedGeneration
	}
	return 0
}

// reusableArtifactImage selects the resolved artifact independently from the
// active release image. A candidate that has passed its build may wait across
// several pre-deploy reconciles while Status.Image intentionally remains the
// previous healthy release.
func reusableArtifactImage(app *appv1alpha1.App, decision appReleaseDecision) (string, bool) {
	if app.Spec.Image != "" {
		return app.Spec.Image, true
	}
	if app.Status.ArtifactImage != "" && !decision.artifactChanged {
		return app.Status.ArtifactImage, true
	}
	if app.Spec.Suspended && app.Status.Image != "" {
		return app.Status.Image, false
	}
	return "", false
}

// requestedReleaseGeneration consumes the identity stamped by backend deploy
// verbs when it is newer than the active release. This closes the race where an
// operational mutation (for example scale 1 -> 2) advances metadata.generation
// after the deploy request but before this first reconcile. A stale annotation
// never masks a later direct CR release edit.
func requestedReleaseGeneration(app *appv1alpha1.App) int64 {
	raw := app.Annotations[appv1alpha1.AnnotationReleaseGeneration]
	generation, err := strconv.ParseInt(raw, 10, 64)
	if err == nil && generation > app.Status.ReleaseGeneration {
		return generation
	}
	return app.Generation
}

func activeRevisionGeneration(app *appv1alpha1.App) int64 {
	if raw, ok := strings.CutPrefix(app.Status.ActiveRevision, "rev-"); ok {
		if generation, err := strconv.ParseInt(raw, 10, 64); err == nil && generation > 0 {
			return generation
		}
	}
	if app.Status.ObservedGeneration > 0 {
		return app.Status.ObservedGeneration
	}
	return app.Generation
}

func releaseGeneration(app *appv1alpha1.App) int64 {
	if app.Status.ReleaseGeneration > 0 {
		return app.Status.ReleaseGeneration
	}
	return activeRevisionGeneration(app)
}

func releaseRevision(app *appv1alpha1.App) string {
	return fmt.Sprintf("rev-%d", releaseGeneration(app))
}

func releaseBuildRevision(app *appv1alpha1.App) string {
	return fmt.Sprintf("gen-%d", releaseGeneration(app))
}
