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

// GitSubmoduleUpdate is the shell both tenant clone phases run at the root of
// a fresh checkout, once the parent commit is checked out, to initialize the
// submodules its root .gitmodules names, as Render does (w8/m89). Each child
// is fetched shallow at the commit the parent pins (never --remote), with a
// relative URL resolved against the parent's remote, and its leftover gitfile
// is removed. Only entries naming a real gitlink are initialized, so a stale
// entry or a stray gitlink builds as it did. GitHub SSH URLs go over HTTPS so
// the caller's credential applies, and no transport but HTTPS is allowed —
// GitHTTPSTransport, the same rule the App's own repo fetch follows.
//
// The caller sets "$@" to its git options (the transport and the credential
// helper) first. The transport is appended again on purpose: .gitmodules is
// tenant-controlled, so this phase does not rely on its caller to pin it. run
// wraps the fetch for the caller's failure handling; refuse is a statement that
// fails the phase with the message given.
//
// SECURITY: a path with a control character is refused outright. Git before
// 2.50.1 mis-writes a submodule path ending in CR, letting a crafted
// repository plant a hook that runs in this phase — the one holding
// GIT_AUTH_TOKEN (CVE-2025-48384). The pinned image is past that, and no
// legitimate path carries one.
func GitSubmoduleUpdate(run, refuse func(string) string) string {
	return `if [ -f .gitmodules ]; then
  export GIT_LITERAL_PATHSPECS=1
  set -- "$@" ` + GitHTTPSTransport + `
  git config --file .gitmodules --name-only --get-regexp '^submodule\..*\.path$' > .git/bex-modules 2>/dev/null || true
  while IFS= read -r key; do
    module="$(git config --file .gitmodules --get "$key" 2>/dev/null)" || continue
    case "$module" in
      *[[:cntrl:]]*) ` + refuse(`the submodule path for $key contains a control character`) + ` ;;
    esac
    case "$(git ls-tree HEAD -- "$module" 2>/dev/null)" in
      "160000 commit "*)
        ` + run(`git "$@" submodule update --init --depth 1 -- "$module" </dev/null`) + `
        rm -f -- "$module/.git" ;;
    esac
  done < .git/bex-modules
fi
`
}
