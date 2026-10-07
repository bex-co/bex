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
	"strconv"

	"github.com/bex-co/bex/lego/types/k8sname"
)

// DefaultRevision is the revision a per-revision Job name substitutes when none
// is resolved, so a revisionless run still gets a stable identity.
const DefaultRevision = "latest"

// RevisionJobName is the deterministic per-revision Job name (DNS-1123, ≤63
// chars) for the build, pre-deploy, and publish planes: re-reconciling the same
// revision reuses that revision's exact Job, and a new revision gets a fresh
// one. Truncation keeps two revisions distinct (k8sname.Fit).
func RevisionJobName(prefix, name, revision string) string {
	rev := revision
	if rev == "" {
		rev = DefaultRevision
	}
	return k8sname.Fit(prefix + name + "-" + rev)
}

// BuildJobName is the build Job's identity. Like ManualCronRunJobName it lives
// in the leaf contract module because both sides of the App CR boundary must
// derive it identically: the operator creates the Job, and bex-api addresses
// that exact Job to cancel a build and to name the image it produced.
func BuildJobName(appName, revision string) string {
	return RevisionJobName("bld-", appName, revision)
}

// BuildRevision spells a generation as the revision the build plane names Jobs
// by. It is the other half of BuildJobName's cross-boundary contract: the
// operator derives it from the App's release generation, and bex-api must
// reproduce it to address the resulting Job.
func BuildRevision(generation int64) string {
	return "gen-" + strconv.FormatInt(generation, 10)
}

// Labels the operator stamps on build artifacts; LabelBuildRevision is on kpack
// artifacts only. bex-api selects a release's kpack Image by them
// (ReleaseBuildLabels) rather than re-deriving its hashed name, which already
// changed under it once (w5/136).
const (
	LabelBuild         = "app.bex.co/build"          // the App's name
	LabelAppUID        = "app.bex.co/app-uid"        // the App's UID: one lifetime of the name
	LabelBuildRevision = "app.bex.co/build-revision" // the release's BuildRevision
)

// ReleaseBuildLabels selects the kpack artifacts of appName's release revision
// in the App lifetime appUID.
func ReleaseBuildLabels(appName, appUID, revision string) map[string]string {
	return map[string]string{LabelBuild: appName, LabelAppUID: appUID, LabelBuildRevision: revision}
}
