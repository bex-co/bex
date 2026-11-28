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
	// ComponentDiskHolder is LabelComponent's value on a disk holder's pod
	// (w6/m559): a Job's pod that keeps a disk service's volume mounted on
	// its node through a Recreate deploy. It runs no app container, so
	// bex-api's reads of the app container leave it out.
	ComponentDiskHolder = "disk-holder"
	// ComponentDiskPrePull is LabelComponent's value on a disk pre-pull's
	// pod (w6/m595): a Job's pod that pulls a code deploy's new image onto
	// the volume's node while the old instance still serves. Its containers
	// never start, and none is the app container.
	ComponentDiskPrePull = "disk-prepull"
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
	// AnnotationRegion records, on an App, Database or KeyValue CR, the
	// Render region it was created with (bex-api's resourcemeta.StampRegion;
	// absent when the create named none). Render's region is fixed at
	// creation, so it never changes.
	AnnotationRegion = "app.bex.co/region"
	// LabelRegion is a pod's private-network region (w4/m712): the operator
	// copies its resource's AnnotationRegion onto every pod that resource
	// runs, and bex-api's private-network policies admit only pods of the
	// same region, as Render's per-region private networks do. A pod without
	// it belongs to the region an unstamped resource reads back.
	LabelRegion = "app.bex.co/region"
	// LabelEgressAppID attributes a pod that must not carry LabelAppID (a
	// pre-deploy Job runs the service's own command but is never a Service
	// backend) to its service's egress. Stamped by the operator's predeploy
	// step, read by its egress meter; it lives here so the two cannot drift
	// apart without importing each other (the meter pulls in eBPF).
	LabelEgressAppID = "app.bex.co/egress-app-id"
	// LabelKeyValue marks the workload (StatefulSet pods, Services) a KeyValue
	// reconciler creates for a store. Stamped by the operator, read back by
	// bex-api (pod logs, pod selection); it lives here so the stamp and the
	// read can never drift apart.
	LabelKeyValue = "app.bex.co/keyvalue"
	// LabelTenant carries a projected App CR's owning tenant id (tea-<id>).
	// Stamped by bex-api (store reconciler, apps.Create) and read by the
	// operator (disk snapshot scoping); it lives here so the stamp and the
	// read can never drift apart.
	LabelTenant = "bex.co/tenant"
	// LabelSandboxID is a sandbox's public id (`sbx-…`), persisted by
	// OpenSandbox as a pod label. Stamped by bex-api (sandbox creates,
	// session-egress policies) and selected by the operator (egress meter
	// allow-list); it lives here so the stamp and the select can never drift
	// apart.
	LabelSandboxID = "bex.co/sandbox-id"
	// LabelAgentSession marks an agent session's sandbox (its session id).
	// Stamped by bex-api (agent-session creates, session-egress policies)
	// and read by the operator egress meter, which excludes it from
	// /v1/sandboxes metering; it lives here so the stamp and the read can
	// never drift apart.
	LabelAgentSession = "bex.co/agent-session"
	// LabelNodePool is the node label infra stamps on every worker, whose
	// value names the pool. bex-api selects it for the tenant pool's
	// outbound IPs and the operator selects it for untrusted workload
	// placement; it lives here so the selects can never drift apart.
	LabelNodePool = "bex.co/pool"
	// NodePoolTenant is LabelNodePool's value on the autoscaled tenant pool,
	// the only pool untrusted tenant workloads may land on.
	NodePoolTenant = "tenant"
)
