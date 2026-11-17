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

// Labels the operator stamps on App CRs and workload pods, and the backend
// reads back. They live here, the one module both sides import, so the stamp
// and the read can never drift apart: every other spelling (backend core's
// PodLabel*/Label* constants, the operator's labelApp/labelWorkspace family,
// the sni-proxy and registry-migrate copies) aliases these. LabelAppUID and
// its build siblings in jobnames.go are the older members of this family.
const (
	// LabelApp marks the workloads bex creates for an App.
	LabelApp = "app.bex.co/app"
	// LabelWorkspace carries the owning workspace (tenant) id on App CRs,
	// Database CRs, KeyValue CRs and their descendant pods, so
	// NetworkPolicy selectors can express "same-workspace" rules.
	LabelWorkspace = "app.bex.co/workspace"
	// LabelRevision ties a pod to the App revision it was created for.
	LabelRevision = "app.bex.co/revision"
	// LabelComponent marks a pod an App's execution step runs in (a
	// pre-deploy Job, a build or publish in the App's namespace), which
	// also carries LabelApp; the App's own instances never carry it.
	LabelComponent = "app.bex.co/component"
	// LabelPreDeploy carries the service name on a pre-deploy Job and its
	// pod (like build's "app.bex.co/build"), so the backend can find the
	// step's pod for its logs.
	LabelPreDeploy = "app.bex.co/predeploy"
	// LabelEnvironment is a pod's environment id, the identity an isolated
	// environment's network boundary selects.
	LabelEnvironment = "app.bex.co/environment-id"
	// LabelNetworkIsolation carries an App's environment id, but ONLY while
	// that environment currently has networkIsolationEnabled=true: unlike
	// LabelEnvironment above (unconditional membership), this label's mere
	// PRESENCE is itself the signal, not just its value.
	LabelNetworkIsolation = "app.bex.co/network-isolation"
)
