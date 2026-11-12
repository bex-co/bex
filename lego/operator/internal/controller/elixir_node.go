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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// elixirRuntime is AppSpec.Runtime for a native Elixir service.
const elixirRuntime = "elixir"

// elixirNodeSubdomain is the pod subdomain a native Elixir service's pods
// get. Render sets RELEASE_DISTRIBUTION=name on every Elixir service, so a
// release starts its node with a long name (`--name <release>`), and Erlang
// takes that name's host from the machine's fully qualified hostname. A pod's
// hostname has no domain, and the node fails to start (`nodistribution`).
// With a subdomain the kubelet writes <pod>.<subdomain>.<namespace>.svc.<cluster
// domain> into /etc/hosts, and the node is <release>@<that name> (w8/272,
// checked on a kind pod with elixir:1.18). No Service of that name is needed
// for /etc/hosts; the name only has to be a DNS label.
const elixirNodeSubdomain = "nodes"

// withElixirNodeHostname gives spec a hostname with a domain when app is a
// native Elixir service. Every other pod is unchanged.
func withElixirNodeHostname(spec *corev1.PodSpec, app *appv1alpha1.App) {
	if app.Spec.Runtime == elixirRuntime {
		spec.Subdomain = elixirNodeSubdomain
	}
}
