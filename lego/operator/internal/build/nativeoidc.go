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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Managed OIDC at build time (docs/ADR116-managed-oidc.md D12, w2/m480).
// render.com/docs/oidc: "OIDC is available at build time for services on any
// native runtime", and "Currently, OIDC auth is not available at build time
// for services that build from a Dockerfile." A native build whose decision
// mounts providers reads each provider's files at the path its running
// instances read them, /var/lib/render/oidc/<file>, with each provider's file
// variable set. The files take the secret-file path (w4/m163): the preparer
// copies them from their Secret into the build's scratch directory, buildctl
// hands each to BuildKit as a secret, and only the authored build RUN mounts
// them, so they never reach a layer, the image config, a build arg or the
// build context. The variables are exported by the env loader after the
// service's own values, so a value the service set itself gives way, as at
// runtime, and the image keeps neither.

const (
	// managedOIDCVolume is the build Pod's volume of the build token Secret;
	// only the preparer mounts it, at managedOIDCPrepareMount.
	managedOIDCVolume       = "managed-oidc"
	managedOIDCPrepareMount = "/managed-oidc"
	// nativeOIDCDir is the preparer's snapshot of the files, beside the
	// generated Dockerfile.
	nativeOIDCDir = "/native/oidc"
)

// nativeOIDCFileEnvPattern is a shell variable name: each provider's file
// variable is written into the env loader as `export NAME=path`.
var nativeOIDCFileEnvPattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// nativeManagedOIDC is the providers whose files o's build command reads:
// none for every builder but native, as on Render.
func nativeManagedOIDC(o Options) []appv1alpha1.ManagedOIDCProvider {
	if o.Builder != BuilderNative || o.ManagedOIDCSecret == "" {
		return nil
	}
	return o.ManagedOIDC
}

// nativeOIDCFiles is the Secret keys, each also its file name, that o's
// providers read, in provider order.
func nativeOIDCFiles(o Options) []string {
	var files []string
	for _, p := range nativeManagedOIDC(o) {
		files = append(files, p.Files()...)
	}
	return files
}

// nativeOIDCSecretID is the BuildKit secret id carrying file. File names are
// the platform's own (ManagedOIDCProviders), in the Secret-key charset.
func nativeOIDCSecretID(file string) string { return "bex-oidc-" + file }

// validateNativeManagedOIDC checks that a Secret and its providers come
// together and that every name spliced into the Dockerfile, the buildctl
// arguments and the loader is one the build can carry unquoted.
func validateNativeManagedOIDC(o Options) error {
	if (o.ManagedOIDCSecret == "") != (len(o.ManagedOIDC) == 0) {
		return fmt.Errorf("build: Managed OIDC needs both a Secret and its providers")
	}
	for _, p := range o.ManagedOIDC {
		if !nativeOIDCFileEnvPattern.MatchString(p.FileEnv) {
			return fmt.Errorf("build: invalid Managed OIDC file variable %q", p.FileEnv)
		}
		for _, f := range p.Files() {
			if !validNativeFileName(f) {
				return fmt.Errorf("build: invalid Managed OIDC file name %q", f)
			}
		}
	}
	return nil
}

// nativeOIDCMounts is the authored build RUN's secret mount flags, one per
// file at its runtime path. required=true makes a missing transport fail the
// build instead of running the command without its token.
func nativeOIDCMounts(o Options) string {
	var flags strings.Builder
	for _, f := range nativeOIDCFiles(o) {
		fmt.Fprintf(&flags, " --mount=type=secret,id=%s,target=%s/%s,required=true",
			nativeOIDCSecretID(f), appv1alpha1.ManagedOIDCMountPath, f)
	}
	return flags.String()
}

// nativeOIDCExports is the env loader's lines that set each provider's file
// variable, after the service's own values.
func nativeOIDCExports(o Options) string {
	var lines strings.Builder
	for _, p := range nativeManagedOIDC(o) {
		fmt.Fprintf(&lines, "export %s=%s\n", p.FileEnv, p.FileEnvPath())
	}
	return lines.String()
}

// nativeOIDCSecretArgs is buildctl's --secret flags for the preparer's
// snapshot of the files.
func nativeOIDCSecretArgs(o Options) []string {
	files := nativeOIDCFiles(o)
	args := make([]string, 0, 2*len(files))
	for _, f := range files {
		args = append(args, "--secret", "id="+nativeOIDCSecretID(f)+",src="+nativeOIDCDir+"/"+f)
	}
	return args
}

// nativeOIDCVolume is the build token Secret's volume, listing each file so
// the preparer does not start before every token exists: the operator copies
// the tokens next to the Job only once bex-api has minted them for this
// build. nil without providers.
func nativeOIDCVolume(o Options) *corev1.Volume {
	files := nativeOIDCFiles(o)
	if len(files) == 0 {
		return nil
	}
	items := make([]corev1.KeyToPath, 0, len(files))
	for _, f := range files {
		items = append(items, corev1.KeyToPath{Key: f, Path: f})
	}
	return &corev1.Volume{
		Name: managedOIDCVolume,
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
			SecretName: o.ManagedOIDCSecret, Items: items,
		}},
	}
}

// withNativeOIDCPreparer has the preparer copy the files into the build's
// scratch directory, the snapshot one build reads even if a refresh rewrites
// the Secret mid-build. Without providers the preparer is unchanged.
func withNativeOIDCPreparer(c *corev1.Container, o Options) {
	files := nativeOIDCFiles(o)
	if len(files) == 0 {
		return
	}
	c.Env = append(c.Env, corev1.EnvVar{Name: "BEX_NATIVE_OIDC_FILES", Value: execution.KubernetesLiteral(strings.Join(files, "\n"))})
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: managedOIDCVolume, MountPath: managedOIDCPrepareMount, ReadOnly: true})
	c.Args[0] += `
mkdir ` + nativeOIDCDir + `
# Names are the platform's own, in the Secret-key charset.
for name in $BEX_NATIVE_OIDC_FILES; do
  cp "` + managedOIDCPrepareMount + `/$name" "` + nativeOIDCDir + `/$name"
done`
}
