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
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// platform_pull_failure_test.go is w8/m156 t004. A release whose build
// retention removed failed with the kubelet's line verbatim:
//
//	image pull is failing: zot.bex-registry.svc:5000/…: Back-off pulling image …
//	failed to resolve image: … not found
//
// It printed a cluster-internal host and never said that the build is gone
// and has to be made again.
func TestPlatformPullFailureReadsInProductTerms(t *testing.T) {
	r := &AppReconciler{Registry: "zot.bex-registry.svc:5000"}
	build := "zot.bex-registry.svc:5000/ws-1/web:gen-8@sha256:" + strings.Repeat("a", 64)

	missing := `Back-off pulling image "` + build + `": ErrImagePull: failed to pull and unpack image "` + build +
		`": failed to resolve image: ` + build + `: not found`
	msg, own := r.platformPullFailure(build, missing)
	if !own {
		t.Fatal("a build in the platform registry was not recognised")
	}
	for _, want := range []string{appv1alpha1.ImagePullStallWaiting, "ws-1/web:gen-8", "no longer stored", "Deploy the commit again"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not say %q", msg, want)
		}
	}
	for _, leak := range []string{"zot.bex-registry.svc", "sha256:", "failed to resolve", "Back-off"} {
		if strings.Contains(msg, leak) {
			t.Errorf("message %q still prints %q", msg, leak)
		}
	}
	// The terminal form swaps the same prefix every stall message uses.
	if got := terminalStallMessage(imagePullBackOff, msg); !strings.HasPrefix(got, appv1alpha1.ImagePullStallFailed+": the build") {
		t.Errorf("terminal message = %q", got)
	}

	// Any other failure of a build is the platform's to fix, and says so
	// without the registry's host or the runtime's error.
	msg, own = r.platformPullFailure(build, `Back-off pulling image "`+build+`": dial tcp 10.0.0.1:5000: i/o timeout`)
	if !own || !strings.Contains(msg, "could not pull this release's build (ws-1/web:gen-8)") || strings.Contains(msg, "zot.bex-registry.svc") || strings.Contains(msg, "dial tcp") {
		t.Errorf("a transient build pull failure reads %q", msg)
	}

	// An image the user names keeps naming itself and the registry's answer.
	if _, own := r.platformPullFailure("docker.io/acme/private:v1", "pull access denied"); own {
		t.Error("an external image was treated as a platform build")
	}
}
