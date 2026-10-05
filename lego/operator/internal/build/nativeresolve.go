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
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// The resolve-native-runtime phase (w8/m51) runs bex's own image after the
// preparer: it reads the decoded env the build will see and the checked-out
// source, picks the toolchain line, rewrites the generated Dockerfile's FROM
// to that line's pin, and narrates the choice into the build log.
const (
	nativeResolveContainer = "resolve-native-runtime"
	// NativeResolveBinary is the entrypoint in bex's image (lego/Dockerfile).
	NativeResolveBinary = "/native-resolve"
	// maxVersionFileBytes bounds what a version file may make the resolver
	// read; real ones are a line, package.json/Gemfile a few KiB.
	maxVersionFileBytes = 1 << 20
)

// ResolveNativeBuild is the resolve phase: env bundle and Dockerfile paths,
// the service's root directory in the checkout, and the runtime. It returns
// the choice it applied; an *UnsupportedVersionError is tenant input.
func ResolveNativeBuild(runtime, bundlePath, dockerfilePath, sourceDir string) (NativeChoice, error) {
	env, err := readEnvBundle(bundlePath)
	if err != nil {
		return NativeChoice{}, err
	}
	choice, err := ResolveNativeVersion(runtime, env, func(name string) ([]byte, bool) {
		return readVersionFile(sourceDir, name)
	})
	if err != nil {
		return NativeChoice{}, err
	}
	dockerfile, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return NativeChoice{}, fmt.Errorf("read generated Dockerfile: %w", err)
	}
	rewritten, err := rewriteNativeBase(string(dockerfile), nativeToolchains[runtime].defaultImage(), choice.Image)
	if err != nil {
		return NativeChoice{}, err
	}
	if err := os.WriteFile(dockerfilePath, []byte(rewritten), 0o600); err != nil {
		return NativeChoice{}, fmt.Errorf("write generated Dockerfile: %w", err)
	}
	return choice, nil
}

// readEnvBundle decodes the preparer's KEY=base64 bundle. A later line wins,
// exactly as the in-image env loader exports them (literals after secrets).
func readEnvBundle(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read env bundle: %w", err)
	}
	defer func() { _ = f.Close() }()
	env := map[string]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	for scanner.Scan() {
		key, encoded, ok := strings.Cut(scanner.Text(), "=")
		if !ok || key == "" {
			continue
		}
		value, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode env bundle entry %s: %w", key, err)
		}
		env[key] = string(value)
	}
	return env, scanner.Err()
}

// readVersionFile reads name from the service's root directory. Only a
// regular file is read: a symlink is ignored, because the refusal message
// quotes what it read and a link could point at the env bundle beside it.
func readVersionFile(sourceDir, name string) ([]byte, bool) {
	path := filepath.Join(sourceDir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, maxVersionFileBytes))
	return raw, err == nil
}

// rewriteNativeBase swaps the generated Dockerfile's single FROM (the default
// line) for the chosen line, refusing anything it did not generate.
func rewriteNativeBase(dockerfile, defaultImage, image string) (string, error) {
	want := "FROM " + defaultImage + "\n"
	if strings.Count(dockerfile, "\nFROM ") != 1 || !strings.Contains(dockerfile, "\n"+want) {
		return "", fmt.Errorf("generated Dockerfile does not carry the expected base %s", defaultImage)
	}
	return strings.Replace(dockerfile, "\n"+want, "\nFROM "+image+"\n", 1), nil
}

// nativeVersionResolver is the resolve phase's container: bex's own image,
// the preparer's non-root identity (it rewrites the 0600 Dockerfile the
// preparer wrote), the checkout read-only, no service account token.
func nativeVersionResolver(o Options) corev1.Container {
	return corev1.Container{
		Name:    nativeResolveContainer,
		Image:   o.NativeResolverImage,
		Command: []string{NativeResolveBinary},
		Env: []corev1.EnvVar{
			{Name: "BEX_NATIVE_RUNTIME", Value: nativeRuntime(o)},
			{Name: "BEX_NATIVE_SOURCE_DIR", Value: boundedSourceDir(o.RootDir)},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "native-build", MountPath: "/native"},
			{Name: "source", MountPath: sourceMount, ReadOnly: true},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                new(int64(65532)),
			RunAsGroup:               new(int64(65532)),
			RunAsNonRoot:             new(true),
			AllowPrivilegeEscalation: new(false),
			ReadOnlyRootFilesystem:   new(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
	}
}
