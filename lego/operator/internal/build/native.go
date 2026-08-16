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
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// nativePreparerImage is digest-pinned (codex round-7 F10 / the ADR055 F7
// digest-pinning deferral's highest-value pin): this initContainer mounts and
// reads the ENTIRE runtime-env secret bundle, so a retagged upstream busybox
// would exfiltrate every tenant build secret. The tag is kept readable for
// humans; the digest is the 1.37.0 multi-arch OCI index.
const nativePreparerImage = "busybox:1.37.0@sha256:9db7b59979c38555a39def84a31fb98b5296952f9e3afd4f6f11f05b07adfab0"

// nativeRuntimeImages retain readable tags but pin their multi-arch manifest
// identities. Patch upgrades are deliberate reviewed changes; a registry retag
// cannot silently alter a privileged tenant build environment.
var nativeRuntimeImages = map[string]string{
	"elixir": "elixir:1.18@sha256:52e8ea10d10e95d74dde312606637e12bc1b1fdf9cfa37d864eacd85fcc16b3c",
	"go":     "golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac",
	"node":   "node:24-bookworm@sha256:934240a162082fd8b8a2f90cd5114446443f1eba1c5378f6687167ca405e6584",
	"python": "python:3.13-bookworm@sha256:62eafe52c91cad83c2c74e630bfde917da8c253673e695665d454def84fc9a13",
	"ruby":   "ruby:3.4-bookworm@sha256:56e0c9fdbf64d090e45072d32f0d3be7f2e392e733444f7d176a50881e6c325a",
	"rust":   "rust:1-bookworm@sha256:0e2bcaef56d041a486784e54104a81aebe0da44bd03019bd70bc0401e42e4a97",
}

func validateNativeOptions(o Options) error {
	if _, ok := nativeRuntimeImages[o.Runtime]; !ok {
		return fmt.Errorf("build: unsupported native runtime %q", o.Runtime)
	}
	if strings.TrimSpace(o.BuildCommand) == "" {
		return fmt.Errorf("build: native runtime %s requires buildCommand", o.Runtime)
	}
	if strings.TrimSpace(o.StartCommand) == "" {
		return fmt.Errorf("build: native runtime %s requires startCommand", o.Runtime)
	}
	return nil
}

// nativeDockerfile translates Render's native runtime contract into a
// reproducible OCI build: select the language toolchain, run the caller's exact
// build command with service env available, and retain the exact start command
// as the image CMD. The generated file is written by an init container and is
// never committed to the tenant repository.
func nativeDockerfile(o Options) string {
	buildScript := `while IFS='=' read -r key encoded; do
  [ -n "$key" ] || continue
  export "$key=$(printf '%s' "$encoded" | base64 -d)"
done < /run/secrets/render-env
` + o.BuildCommand
	run := shellJSON(buildScript)
	start := shellJSON(o.StartCommand)
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.7
FROM %s
WORKDIR /opt/render/project/src
COPY . .
RUN --mount=type=secret,id=render-env,target=/run/secrets/render-env %s
ENV PORT=10000
CMD %s
`, nativeRuntimeImages[o.Runtime], run, start)
}

func shellJSON(command string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode([]string{"/bin/bash", "-lc", command})
	return strings.TrimSpace(out.String())
}

// nativeBuildPreparer materializes the generated Dockerfile and a base64 env
// bundle into an EmptyDir shared with buildkit. The env bundle enters BuildKit
// through a secret mount, so neither literal nor OpenBao-backed values appear in
// image metadata or the generated Dockerfile.
func nativeBuildPreparer(o Options) corev1.Container {
	keys := make([]string, 0, len(o.BuildEnv))
	env := make([]corev1.EnvVar, 0, len(o.BuildEnv)+2)
	for _, item := range o.BuildEnv {
		if item.Name == "" || item.ValueFrom != nil || item.Name == "PORT" || strings.HasPrefix(item.Name, "BEX_NATIVE_") {
			continue
		}
		keys = append(keys, item.Name)
		env = append(env, corev1.EnvVar{Name: item.Name, Value: item.Value})
	}
	sort.Strings(keys)
	env = append(env,
		corev1.EnvVar{Name: "BEX_NATIVE_DOCKERFILE", Value: nativeDockerfile(o)},
		corev1.EnvVar{Name: "BEX_NATIVE_LITERAL_KEYS", Value: strings.Join(keys, "\n")},
	)

	mounts := []corev1.VolumeMount{{Name: "native-build", MountPath: "/native"}}
	if o.RuntimeEnvSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{Name: "runtime-env", MountPath: "/runtime-env", ReadOnly: true})
	}
	return corev1.Container{
		Name:    "prepare-native-build",
		Image:   nativePreparerImage,
		Command: []string{"sh", "-eu", "-c"},
		Args: []string{`umask 077
printf '%s' "$BEX_NATIVE_DOCKERFILE" > /native/Dockerfile
: > /native/render-env
if [ -d /runtime-env ]; then
  for path in /runtime-env/*; do
    [ -f "$path" ] || continue
    key="$(basename "$path")"
    encoded="$(base64 < "$path" | tr -d '\n')"
    printf '%s=%s\n' "$key" "$encoded" >> /native/render-env
  done
fi
for key in $BEX_NATIVE_LITERAL_KEYS; do
  encoded="$(printenv "$key" | base64 | tr -d '\n')"
  printf '%s=%s\n' "$key" "$encoded" >> /native/render-env
done`},
		Env:          env,
		VolumeMounts: mounts,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                ptr(int64(65532)),
			RunAsGroup:               ptr(int64(65532)),
			RunAsNonRoot:             ptr(true),
			AllowPrivilegeEscalation: ptr(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	}
}
