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

import "strings"

// GitHubSSHRepoPrefixes are the SSH spellings of a GitHub repository URL a
// tenant may give for spec.repo, the forms GitHub's clone button offers. Tenant
// fetches allow no transport but HTTPS, so the operator fetches each of these
// as https://github.com/… (url.<base>.insteadOf), where the GitHub credential
// applies and no SSH host trust is needed; bex-api admits an SSH repo URL only
// in these forms (w8/130). One definition, so admission and the rewrite cannot
// drift; the dashboard's client check (dashboard/src/common/lib/utils/git-url.ts)
// mirrors it and must change with it.
var GitHubSSHRepoPrefixes = []string{"git@github.com:", "ssh://git@github.com/", "ssh://github.com/"}

// IsGitHubSSHRepo reports whether repo is spelled in one of
// GitHubSSHRepoPrefixes.
func IsGitHubSSHRepo(repo string) bool {
	for _, prefix := range GitHubSSHRepoPrefixes {
		if strings.HasPrefix(repo, prefix) {
			return true
		}
	}
	return false
}
