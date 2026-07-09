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

package core

// config.go holds the validation rules shared by the two config features
// (internal/secrets env vars + secret files, internal/envgroups) — both write
// into per-app Kubernetes Secrets, so both must reject names that aren't legal
// Secret keys before the write reaches the apiserver with a cryptic error.

// ValidEnvKey reports whether k is a C-locale environment variable name
// ([A-Za-z_][A-Za-z0-9_]*): what a shell and Kubernetes' Secret-key validation
// both accept. Rejecting the rest keeps a bad name from failing the Secret write
// with a cryptic error later.
func ValidEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// ValidSecretFileName reports whether name is a legal Kubernetes Secret key
// ([-._a-zA-Z0-9]+, not "."/".."): the file is mounted at /etc/secrets/<name>, so
// a name outside this set (a path, in particular) would fail the Secret write.
func ValidSecretFileName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		switch {
		case r == '_', r == '-', r == '.':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
