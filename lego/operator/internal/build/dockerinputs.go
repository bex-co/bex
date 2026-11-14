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
	"cmp"
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
// NativeFilesSecrets — plus the literal BuildEnv entries, which win last as they
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
	dockerBuildContextEnv  = "BEX_BUILD_CONTEXT"
)

// contextFilesContainer copies a Dockerfile build's secret files into the
// root of its build context (dockerContextFilesStep).
const contextFilesContainer = "context-files"

// Build-pod volume names shared by the native and Dockerfile paths. They are
// live Kubernetes names, so renaming one changes every build Job's spec.
const (
	sourceVolume      = "source"
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

// dockerContextDir is the directory BuildKit reads as the build context:
// DockerContext when set, else RootDir, both repository-relative.
func dockerContextDir(o Options) string {
	return boundedSourceDir(cmp.Or(o.DockerContext, o.RootDir))
}

// copiesContextFiles reports whether the build is a Dockerfile build that
// has secret files to put in its context.
func copiesContextFiles(o Options) bool {
	return o.Builder != BuilderNative && len(o.NativeFilesSecrets) > 0 && len(o.NativeFiles) > 0
}

// dockerContextFilesStep puts each secret file in the root of the Docker
// build context, where Render puts it as well as passing it as a build
// secret: "Render also copies your secret files into the root of your Docker
// build context" (render.com/docs/docker-secrets, read 2026-11-11; w8/253). A
// Dockerfile that reads an uploaded .npmrc with `COPY .npmrc …` therefore
// builds, and one that copies its whole context has the files in its image,
// which is why Render tells authors to list them in .dockerignore. The build
// secrets stay as they are: they are read from the private snapshot, not from
// the context, so a name in .dockerignore still mounts.
//
// It is its own restricted container, between the clone and BuildKit: the
// BuildKit container keeps its read-only source mount, and the clone, which
// runs git over tenant input with the repository credential, never sees the
// files.
func dockerContextFilesStep(o Options, image string) corev1.Container {
	return corev1.Container{
		Name:    contextFilesContainer,
		Image:   image,
		Command: []string{"sh", "-eu", "-c", dockerContextFilesScript()},
		Env: []corev1.EnvVar{
			{Name: dockerBuildContextEnv, Value: execution.KubernetesLiteral(dockerContextDir(o))},
			{Name: dockerBuildFilesEnv, Value: execution.KubernetesLiteral(strings.Join(o.NativeFiles, "\n"))},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: sourceVolume, MountPath: sourceMount},
			{Name: secretFilesVolume, MountPath: dockerBuildFilesMount, ReadOnly: true},
		},
		SecurityContext: execution.RestrictedContainerSecurityContext(),
		Resources:       sourceStepResources(),
	}
}

// dockerContextFilesScript is the copy. The repository is tenant input, so
// nothing in it may steer a write:
//   - The step enters the context and writes relative to it, so no path is
//     resolved a second time. The directory it entered must lie inside the
//     source mount; a context that is a link out of the repository, or is no
//     directory at all, is refused as the tenant's error.
//   - A link or file of the secret's name is removed before the copy, so the
//     copy never follows a link to its target.
//   - A real directory of that name is the repository's own and is kept, the
//     rule a native service's root follows (w8/m154).
//
// The file is 0644 like any file of a checkout: BuildKit reads the context
// as another user.
func dockerContextFilesScript() string {
	return `cd "$` + dockerBuildContextEnv + `" || {
  echo "the Docker build context is not a directory of the repository" >&2
  exit ` + fmt.Sprint(ExitTenantError) + `
}
case "$(pwd -P)/" in
  ` + sourceMount + `/*) ;;
  *)
    echo "the Docker build context is a link that leaves the repository" >&2
    exit ` + fmt.Sprint(ExitTenantError) + `
    ;;
esac
for name in $` + dockerBuildFilesEnv + `; do
  if [ -d "./$name" ] && [ ! -L "./$name" ]; then
    continue
  fi
  rm -f "./$name"
  cp "` + dockerBuildFilesMount + `/$name" "./$name"
  chmod 0644 "./$name"
done
`
}
