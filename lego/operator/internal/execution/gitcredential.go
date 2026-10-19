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

package execution

import (
	"strings"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// GitHubCredentialHelper is the inline `git -c` credential-helper expression
// both tenant clone phases — the build plane's clone initContainer
// (internal/build) and the static publish plane's direct-clone container
// (internal/publish) — splice into their fetch command when a CloneSecret
// supplies GIT_AUTH_TOKEN. One guarded definition (w1/m80): the two planes
// carried byte-identical copies with the rationale below on only one of them,
// so an edit to the other copy had no signal that the string is load-bearing.
//
// SECURITY: the credential helper is host-bound — it answers only when git
// asks for github.com credentials (the "host=" line of git's credential
// protocol). bex-api only mints a GIT_AUTH_TOKEN for a structurally
// verified github.com origin, so this is defense in depth: even if a
// crafted REPO caused git to connect elsewhere, the helper returns nothing
// and the token never leaves for a non-GitHub host.
const GitHubCredentialHelper = `credential.helper='!f() { [ "$1" = get ] || exit 0; h=; while IFS= read -r l; do [ -z "$l" ] && break; case "$l" in host=*) h=${l#host=};; esac; done; [ "$h" = github.com ] || exit 0; echo "username=x-access-token"; echo "password=$GIT_AUTH_TOKEN"; }; f'`

// GitHTTPSTransport is the `git -c` option set every tenant fetch carries — the
// build plane's and the publish plane's parent fetches and the submodule phase
// (GitSubmoduleUpdate): no transport but HTTPS is allowed, and each GitHub SSH
// spelling bex-api admits (appv1alpha1.GitHubSSHRepoPrefixes) is fetched as
// https://github.com/…, so GitHubCredentialHelper applies and the clone needs
// no SSH host trust (w8/130). Before it, only submodules were rewritten, and an
// accepted git@github.com: repo failed every build on host-key verification.
var GitHTTPSTransport = gitHTTPSTransport()

func gitHTTPSTransport() string {
	opts := make([]string, 0, 2+len(appv1alpha1.GitHubSSHRepoPrefixes))
	opts = append(opts, "-c protocol.allow=never", "-c protocol.https.allow=always")
	for _, prefix := range appv1alpha1.GitHubSSHRepoPrefixes {
		opts = append(opts, "-c url.https://github.com/.insteadOf="+prefix)
	}
	return strings.Join(opts, " ")
}
