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
		branch = "main"
	}

	artifactInput := artifactIdentityInput{
		Repo:                       spec.Repo,
		Image:                      spec.Image,
		RootDir:                    spec.RootDir,
		DockerfilePath:             spec.DockerfilePath,
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
	port := spec.Port
	if port == 0 {
		port = 3000
	}
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
	legacyBackfill  bool
}

// prepareAppReleaseDecision mutates only operator-owned status. A legacy App
// with an active revision is adopted as-is: this is what lets an upgrade repair
// the production scale incident (generation advanced, old image still serving)
// without treating the upgrade/current operational generation as a deploy.
func prepareAppReleaseDecision(app *appv1alpha1.App) appReleaseDecision {
	desired := desiredAppReleaseIdentity(app.Spec)
	legacy := app.Status.ReleaseFingerprint == "" && app.Status.ActiveRevision != ""
	if legacy {
		app.Status.ArtifactFingerprint = desired.artifact
		app.Status.ArtifactImage = app.Status.Image
		app.Status.ReleaseFingerprint = desired.release
		app.Status.ReleaseGeneration = activeRevisionGeneration(app)
		return appReleaseDecision{desired: desired, legacyBackfill: true}
	}

	artifactChanged := app.Status.ArtifactFingerprint != desired.artifact
	releaseChanged := app.Status.ReleaseFingerprint != desired.release
	if releaseChanged {
		app.Status.ReleaseFingerprint = desired.release
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
