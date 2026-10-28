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

package build

import (
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// A Dockerfile build receives the service's environment the way Render's does
// (w1/m178, docs/render-artifacts/docker-build-env.md): every effective env var
// as a build arg with the same key and value, and every secret file as a
// BuildKit build secret whose id is the filename with each character outside
// [A-Za-z0-9_-] replaced by "_" (so `RUN --mount=type=secret,id=_env` reads
// `.env` at /run/secrets/_env).
//
// The inputs are the same App-owned projections a native build mounts —
// RuntimeEnvSecret (linked groups, then the service's own Secret) and
// NativeFilesSecret — plus the literal BuildEnv entries, which win last as they
// do at runtime. Secret-backed values never enter the Job spec: the BuildKit
// container reads them from its mounts and appends each as a discrete
// `--opt build-arg:K=V` positional parameter, and copies the secret files into
// a private EmptyDir so one build reads one snapshot even if a later reconcile
// rewrites the projection mid-build.
const (
	dockerBuildEnvMount    = "/bex/build-env"
	dockerBuildFilesMount  = "/bex/build-files"
	dockerBuildInputsMount = "/bex/build-inputs"
	dockerBuildArgEnv      = "BEX_BUILD_ARG_"
	dockerBuildArgKeysEnv  = "BEX_BUILD_ARG_KEYS"
	dockerBuildFilesEnv    = "BEX_BUILD_FILES"
)

// Build-pod volume names shared by the native and Dockerfile paths. They are
// live Kubernetes names, so renaming one changes every build Job's spec.
const (
	runtimeEnvVolume  = "runtime-env"
	secretFilesVolume = "native-files"
	buildInputsVolume = "build-inputs"
)

var renderSecretIDUnsafe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// renderSecretFileID is the BuildKit secret id Render gives a secret file in a
// Docker build: the filename with every character outside [A-Za-z0-9_-]
// replaced by "_" (".env" -> "_env", "credentials.json" -> "credentials_json").
func renderSecretFileID(name string) string {
	return renderSecretIDUnsafe.ReplaceAllString(name, "_")
}

// validateDockerfileInputs adds Render's id rule to the shared secret-file
// checks: two files that map to one id cannot both be addressed, so the build
// fails naming both rather than silently shadowing one of them.
func validateDockerfileInputs(o Options) error {
	if err := validateSecretFiles(o); err != nil {
		return err
	}
	seen := make(map[string]string, len(o.NativeFiles))
	for _, name := range o.NativeFiles {
		id := renderSecretFileID(name)
		if prior, ok := seen[id]; ok {
			return fmt.Errorf("build: secret files %q and %q both become Docker build secret id %q; rename one of them", prior, name, id)
		}
		seen[id] = name
	}
	return nil
}

// dockerBuildInputsEnv carries the literal build args (indexedLiteralEnv) plus
// the key and file lists the input script walks.
func dockerBuildInputsEnv(o Options, literals []corev1.EnvVar) []corev1.EnvVar {
	if len(literals) == 0 && len(o.NativeFiles) == 0 {
		return nil
	}
	env, keys := indexedLiteralEnv(literals, dockerBuildArgEnv)
	return append(env,
		corev1.EnvVar{Name: dockerBuildArgKeysEnv, Value: keys},
		corev1.EnvVar{Name: dockerBuildFilesEnv, Value: execution.KubernetesLiteral(strings.Join(o.NativeFiles, "\n"))},
	)
}

// dockerBuildSecretArgs names each copied secret file to buildctl under its
// Render id. Names are validated to the Secret-key charset, so the CSV field
// needs no quoting.
func dockerBuildSecretArgs(o Options) []string {
	args := make([]string, 0, 2*len(o.NativeFiles))
	for _, name := range o.NativeFiles {
		args = append(args, "--secret", "id="+renderSecretFileID(name)+",src="+dockerBuildInputsMount+"/files/"+name)
	}
	return args
}

// dockerBuildInputsScript runs in the BuildKit container before buildctl. It
// appends every Secret-backed env var, then every literal (later --opt wins in
// buildctl, so literals override Secret keys as they do at runtime), and copies
// the secret files into the private EmptyDir the --secret flags point at. A
// "." sentinel keeps a value's trailing newlines through command substitution;
// for a literal, %?. also drops printenv's own final newline. Empty when the
// build has no env or files, so such a Job is unchanged.
func dockerBuildInputsScript(o Options, literals []corev1.EnvVar) string {
	if o.RuntimeEnvSecret == "" && len(literals) == 0 && len(o.NativeFiles) == 0 {
		return ""
	}
	return `umask 077
if [ -d ` + dockerBuildEnvMount + ` ]; then
  for path in ` + dockerBuildEnvMount + `/* ` + dockerBuildEnvMount + `/.[!.]*; do
    [ -f "$path" ] || continue
    value="$(cat "$path"; printf '.')"
    set -- "$@" --opt "build-arg:${path##*/}=${value%.}"
  done
fi
i=0
for key in ${` + dockerBuildArgKeysEnv + `:-}; do
  value="$(printenv "` + dockerBuildArgEnv + `$i"; printf '.')"
  i=$((i + 1))
  set -- "$@" --opt "build-arg:$key=${value%?.}"
done
if [ -n "${` + dockerBuildFilesEnv + `:-}" ]; then
  mkdir -p ` + dockerBuildInputsMount + `/files
  for name in $` + dockerBuildFilesEnv + `; do
    cp "` + dockerBuildFilesMount + `/$name" "` + dockerBuildInputsMount + `/files/$name"
  done
fi
`
}
