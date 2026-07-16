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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespacedSecretCacheOptions keeps the manager's Secret informer inside the
// tenant apps namespace while leaving every other cached kind cluster-scoped.
// The operator's ClusterRole deliberately cannot read Secrets; a Role grants
// Secret access only where tenant Apps, Databases, and KeyValues live. Without
// this per-kind scope, an Owns(Secret) watch asks for a cluster-wide list and
// prevents the entire shared cache (including the App controller) from starting.
//
// Build-plane, per-App registry, and admission-webhook Secret reads use uncached
// clients, so their separately scoped build/platform Roles are unaffected by
// this cache boundary.
func NamespacedSecretCacheOptions(namespace string) cache.Options {
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {
				Namespaces: map[string]cache.Config{namespace: {}},
			},
		},
	}
}
