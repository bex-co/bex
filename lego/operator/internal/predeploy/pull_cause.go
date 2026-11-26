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

package predeploy

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// RegistryRefusal says why a registry refused image, as one of the causes
// Render lists for an Image Pull Failed event: "Image not found (404 error)",
// "Authentication error" and "Rate limited (429 error)", each with what to do
// (render.com/docs/deploying-an-image). status is the registry's HTTP answer.
// credentialed says the pull carried the service's selected registry
// credential: a 401 or 403 then means the registry refused that credential,
// which is what the owner must fix (Docker Hub answers 401, GHCR 403, even for
// a public image; w4/m301). Any other status keeps naming itself. Worded from
// the shared templates, which the dashboard words in its own language (w4/566).
func RegistryRefusal(image string, status int, credentialed bool) string {
	imageParam := appv1alpha1.DeployReasonParam{Name: "image", Value: image}
	hostParam := appv1alpha1.DeployReasonParam{Name: "host", Value: RegistryHost(image)}
	statusParam := appv1alpha1.DeployReasonParam{Name: "status", Value: strconv.Itoa(status)}
	switch status {
	case http.StatusNotFound:
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageNotFound, imageParam)
	case http.StatusUnauthorized, http.StatusForbidden:
		if credentialed {
			return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageCredential, hostParam, imageParam, statusParam)
		}
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageAccess, imageParam, hostParam)
	case http.StatusTooManyRequests:
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageRegistryRateLimited, hostParam, imageParam)
	}
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageRead, imageParam, statusParam)
}

// UserPullFailure is a node's failed pull of an image the user supplied,
// worded as the Render cause the kubelet's message names, as a refused image
// metadata read is (RegistryRefusal). A registry that answered 5xx or could
// not be reached is Render's "Registry unavailable". ok is false when the
// message names no cause (a back-off line without the error behind it), or
// the pod reports no image to name: the caller keeps the kubelet's own text.
// credentialed is RegistryRefusal's.
func UserPullFailure(image, kubelet string, credentialed bool) (msg string, ok bool) {
	status := kubeletPullStatus(kubelet)
	switch {
	case status == 0 || image == "":
		return "", false
	case status == http.StatusServiceUnavailable:
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageRegistryUnavailable,
			appv1alpha1.DeployReasonParam{Name: "host", Value: RegistryHost(image)},
			appv1alpha1.DeployReasonParam{Name: "image", Value: image}), true
	}
	return RegistryRefusal(image, status, credentialed), true
}

// kubeletPullAnswers are the registry answers a kubelet's pull error carries
// (containerd's resolver and fetcher wording, the CRI's "registry is
// unavailable", and Docker Hub's and GHCR's own codes), by the HTTP status
// each stands for; 503 stands for every 5xx and network failure. The first
// family that matches wins, so a throttled pull is never read as a refusal.
// A refusal is also what makes a pull failure permanent
// (PermanentPullFailure).
var kubeletPullAnswers = []struct {
	status  int
	answers []string
}{
	{http.StatusTooManyRequests, []string{"429 too many requests", "toomanyrequests", "rate limit"}},
	{http.StatusNotFound, []string{"code = notfound", "manifest unknown", "manifest_unknown", ": not found", "404 not found"}},
	{http.StatusForbidden, []string{"403 forbidden", "denied:"}},
	// Docker Hub answers a missing repository as it does a private one:
	// "pull access denied, repository does not exist or may require
	// authorization … insufficient_scope" (w8/131), as its metadata read
	// answers 401.
	{http.StatusUnauthorized, []string{"401 unauthorized", "unauthorized", "authentication required", "pull access denied",
		"repository does not exist", "insufficient_scope"}},
	{http.StatusServiceUnavailable, []string{"registry is unavailable", "500 internal server error", "502 bad gateway",
		"503 service unavailable", "504 gateway timeout", "no such host", "i/o timeout", "connection refused",
		"connection reset", "tls handshake timeout", "network is unreachable", "context deadline exceeded", "server misbehaving"}},
}

// kubeletPullStatus is the registry answer a kubelet's pull error names
// (kubeletPullAnswers), or 0 when it names none.
func kubeletPullStatus(kubelet string) int {
	lower := strings.ToLower(kubelet)
	for _, family := range kubeletPullAnswers {
		if slices.ContainsFunc(family.answers, func(answer string) bool { return strings.Contains(lower, answer) }) {
			return family.status
		}
	}
	return 0
}

// RegistryHost is the registry an image reference names: its first path
// segment when that looks like a host, else Docker Hub.
func RegistryHost(image string) string {
	if host, _, ok := strings.Cut(image, "/"); ok && (strings.ContainsAny(host, ".:") || host == "localhost") {
		return host
	}
	return "docker.io"
}
