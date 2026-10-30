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

// DatastorePriorityClassName is the PriorityClass every managed-datastore Pod
// runs under: CNPG Postgres instances and PgBouncer poolers, and the Valkey
// StatefulSet plus its internal proxy (w2/m247). Its value is 0, the same
// priority a Pod with no class gets, so it changes nothing about scheduling or
// preemption. It exists to be NAMED: the workspace's compute ResourceQuota
// (store.NamespaceReconciler) is scoped `PriorityClass NotIn [this]`, so
// datastore Pods are not charged against the CPU/memory budget that bounds a
// workspace's services. Datastores are bounded instead by their own count caps
// and per-instance billing, as on Render, which has no per-workspace compute
// quota. The class itself ships in deploy/gitops/base/datastore-priority.yaml.
//
// It lives in the leaf contract module because both sides of the CR boundary
// must spell it identically: the operator stamps it on the Pods, bex-api scopes
// the quota by it. A drift would silently charge every datastore again.
const DatastorePriorityClassName = "bex-datastore"
