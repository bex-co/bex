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

// Namespace regime labels. bex-api's NamespaceReconciler stamps LabelRegime on
// each workspace's hosting and sandbox namespaces (ADR043 D2 — both untrusted,
// opposite default network postures), and on the OpenSandbox create metadata
// that selects the resulting BatchSandbox and Pod for sandbox-only network
// policy. The operator (sandbox namespace registry) and bex-api (agent
// sessions, sandbox metering) read it back. The key and its values live here,
// the one module both sides import, so the stamp and the reads can never drift
// apart; every other spelling (backend store's RegimeLabel family, the
// sandbox and agentsession metadata keys, the registry reconciler's label)
// aliases these.
const (
	// LabelRegime distinguishes a workspace's hosting namespace from its
	// sandbox namespace.
	LabelRegime = "app.bex.co/regime"
	// RegimeSandbox is the LabelRegime value selecting sandbox posture.
	RegimeSandbox = "sandbox"
	// RegimeHosting is the LabelRegime value selecting hosting posture.
	RegimeHosting = "hosting"
)
