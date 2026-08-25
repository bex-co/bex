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
	"reflect"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// SpecIdentityClass is the exhaustive policy for deciding what an AppSpec
// mutation means. Artifact fields can require source access/a build. Release
// fields can require pre-deploy and a workload rollout. Operational fields are
// reconciled against the current release and must do neither.
//
// The classification lives in the CRD contract module rather than the operator
// because both sides of the control plane need it: the operator to fingerprint
// artifact/release identity (internal/controller/release_identity.go), and
// bex-api to decide whether a spec patch it is about to make will roll a new
// release — and therefore owes the user a deploy-history row (w6/m51). Two
// copies of this policy would drift, and a drifted copy is exactly the bug
// w6/m51 was filed for: a rollout nothing recorded.
type SpecIdentityClass uint8

const (
	IdentityOperational SpecIdentityClass = 0
	IdentityArtifact    SpecIdentityClass = 1 << 0
	IdentityRelease     SpecIdentityClass = 1 << 1
)

// AppSpecIdentityClasses deliberately names every AppSpec field. The operator's
// guard tests fail when the CRD contract gains a field without a deploy-
// semantics decision here, and when a field's real fingerprint behavior stops
// matching its declared class.
var AppSpecIdentityClasses = map[string]SpecIdentityClass{
	"DisplayName":                IdentityOperational,
	"Type":                       IdentityRelease,
	"Schedule":                   IdentityOperational,
	"Command":                    IdentityRelease,
	"RunAt":                      IdentityOperational,
	"CancelRun":                  IdentityOperational,
	"PublishPath":                IdentityRelease,
	"Routes":                     IdentityOperational,
	"Headers":                    IdentityOperational,
	"Repo":                       IdentityArtifact | IdentityRelease,
	"Image":                      IdentityArtifact | IdentityRelease,
	"RootDir":                    IdentityArtifact | IdentityRelease,
	"BuildFilter":                IdentityOperational,
	"DockerfilePath":             IdentityArtifact | IdentityRelease,
	"DockerContext":              IdentityArtifact | IdentityRelease,
	"Branch":                     IdentityArtifact | IdentityRelease,
	"BuildCommit":                IdentityArtifact | IdentityRelease,
	"CloneSecret":                IdentityOperational,
	"ExternalRegistryPullSecret": IdentityArtifact | IdentityRelease,
	"RegistryCredentialID":       IdentityArtifact | IdentityRelease,
	"Runtime":                    IdentityArtifact | IdentityRelease,
	"BuildCommand":               IdentityArtifact | IdentityRelease,
	"StartCommand":               IdentityArtifact | IdentityRelease,
	"Builder":                    IdentityArtifact | IdentityRelease,
	"Replicas":                   IdentityOperational,
	"Port":                       IdentityRelease,
	"Env":                        IdentityArtifact | IdentityRelease,
	"EnvFromSecret":              IdentityArtifact | IdentityRelease,
	"EnvFromSecrets":             IdentityRelease,
	"FilesFromSecrets":           IdentityRelease,
	"HealthCheckPath":            IdentityRelease,
	"MaxShutdownDelaySeconds":    IdentityRelease,
	"AutoDeploy":                 IdentityOperational,
	"NotifyOnFail":               IdentityOperational,
	"NotificationsToSend":        IdentityOperational,
	"PreDeployCommand":           IdentityRelease,
	"IdleTTLSeconds":             IdentityOperational,
	"RestartedAt":                IdentityArtifact | IdentityRelease,
	"Suspended":                  IdentityOperational,
	"Autoscaling":                IdentityOperational,
	// A disk is a release input because attaching, detaching, or remounting one
	// rewrites the pod template — Render redeploys the service for exactly that
	// reason. Its SIZE deliberately is not: a grow is applied to the live volume
	// online, and rolling the pod for it would turn Render's no-downtime resize
	// into an outage. See the projection in desiredAppReleaseIdentity.
	"Disk":                   IdentityRelease,
	"Tier":                   IdentityRelease,
	"Host":                   IdentityOperational,
	"Expose":                 IdentityOperational,
	"Subdomain":              IdentityOperational,
	"Hosts":                  IdentityOperational,
	"HostRedirects":          IdentityOperational,
	"SubdomainPolicy":        IdentityOperational,
	"IPAllowList":            IdentityOperational,
	"IPAllowListEntries":     IdentityOperational,
	"EnvironmentIPAllowList": IdentityOperational,
	"MaintenanceMode":        IdentityOperational,
}

// SpecRollsRelease reports whether moving an App from the before spec to the
// after spec changes any field the operator treats as artifact or release
// identity — that is, whether the patch will make the operator rebuild the
// image or roll new pods rather than reconcile the running release in place.
//
// This is the deploy-history gate (w6/m51): a spec patch that answers true is a
// real rollout a user must be able to see, retry, and roll back, so its writer
// owes it a deploy row. A patch that answers false changed only operational
// intent (scale, autoDeploy, custom domains, notification policy) and must NOT
// mint a spurious deploy.
//
// Disk is compared through the same mount-path-only projection the operator's
// release fingerprint uses: growing a disk resizes the live volume online, and
// recording that as a deploy would imply an outage that never happens.
func SpecRollsRelease(before, after AppSpec) bool {
	beforeValue := reflect.ValueOf(before)
	afterValue := reflect.ValueOf(after)
	specType := beforeValue.Type()
	for i := range specType.NumField() {
		name := specType.Field(i).Name
		if AppSpecIdentityClasses[name] == IdentityOperational {
			continue
		}
		if name == "Disk" {
			if diskRollsRelease(before.Disk, after.Disk) {
				return true
			}
			continue
		}
		// Semantic equality so a nil slice/map and its empty counterpart — which
		// list mutations like "remove the last linked env group" produce
		// interchangeably — are not read as a change and do not open a deploy
		// row for a patch that changed nothing.
		if !apiequality.Semantic.DeepEqual(beforeValue.Field(i).Interface(), afterValue.Field(i).Interface()) {
			return true
		}
	}
	return false
}

// diskRollsRelease compares only what a disk contributes to the pod template:
// whether there is one, and where it is mounted.
func diskRollsRelease(before, after *DiskSpec) bool {
	if (before == nil) != (after == nil) {
		return true
	}
	return before != nil && before.MountPath != after.MountPath
}
