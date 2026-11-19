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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// oidcBuildOptions is a native service build whose decision mounts AWS and
// GCP, the one provider with a credential configuration beside its token.
func oidcBuildOptions() Options {
	o := nativeOptions()
	o.ManagedOIDCSecret = "bld-web-oidc"
	o.ManagedOIDC = appv1alpha1.ManagedOIDCProvidersNamed([]string{"aws", "gcp"})
	return o
}

func volumeNamed(spec corev1.PodSpec, name string) *corev1.Volume {
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == name {
			return &spec.Volumes[i]
		}
	}
	return nil
}

// w2/m480, docs/ADR116-managed-oidc.md D12: a native build reads each
// provider's files where its instances do, through transient secret mounts
// on the authored RUN alone, with the file variables exported after the
// service's own values; only the preparer reads the token Secret.
func TestNativeBuildMountsManagedOIDCFilesForTheBuildCommandOnly(t *testing.T) {
	o := oidcBuildOptions()
	if err := validateNativeOptions(o); err != nil {
		t.Fatal(err)
	}
	df := nativeDockerfile(o)
	want := " --mount=type=secret,id=bex-oidc-aws-token,target=/var/lib/render/oidc/aws-token,required=true" +
		" --mount=type=secret,id=bex-oidc-gcp-token,target=/var/lib/render/oidc/gcp-token,required=true" +
		" --mount=type=secret,id=bex-oidc-gcp-credentials.json,target=/var/lib/render/oidc/gcp-credentials.json,required=true" + nativeXDGCacheMount + " ["
	if !strings.Contains(df, nativeEnvRunMount+want) {
		t.Fatalf("authored RUN missing the token mounts %q:\n%s", want, df)
	}
	if strings.Count(df, "bex-oidc-") != 3 {
		t.Fatalf("token mounts outside the authored RUN:\n%s", df)
	}
	// The variables come after the decode loop, so the service's own value
	// gives way, and inside the loader only, so the image keeps neither.
	exports := "done < " + nativeEnvSecretPath + "\n" +
		"export AWS_WEB_IDENTITY_TOKEN_FILE=/var/lib/render/oidc/aws-token\n" +
		"export GOOGLE_APPLICATION_CREDENTIALS=/var/lib/render/oidc/gcp-credentials.json\n" + nativeEnvLoaderEOF + "\n"
	if !strings.Contains(df, exports) {
		t.Fatalf("env loader missing the file variables after the service's values:\n%s", df)
	}
	for _, reject := range []string{"ENV AWS_", "ENV GOOGLE_", "ARG AWS_", "ARG GOOGLE_", "COPY /native/oidc", "/native/oidc"} {
		if strings.Contains(df, reject) {
			t.Fatalf("token transport must not use %q:\n%s", reject, df)
		}
	}

	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	vol := volumeNamed(spec, managedOIDCVolume)
	if vol == nil || vol.Secret == nil || vol.Secret.SecretName != "bld-web-oidc" || vol.Secret.Optional != nil {
		t.Fatalf("token volume = %+v, want the required Secret bld-web-oidc", vol)
	}
	keys := make([]string, 0, len(vol.Secret.Items))
	for _, item := range vol.Secret.Items {
		keys = append(keys, item.Key+"="+item.Path)
	}
	if want := []string{"aws-token=aws-token", "gcp-token=gcp-token", "gcp-credentials.json=gcp-credentials.json"}; !slices.Equal(keys, want) {
		t.Fatalf("token volume items = %v, want %v: the preparer waits for every file", keys, want)
	}
	buildkit := containerByName(spec.InitContainers, "buildkit")
	for _, f := range []string{"aws-token", "gcp-token", "gcp-credentials.json"} {
		if !containsPair(buildkit.Args, "--secret", "id=bex-oidc-"+f+",src=/native/oidc/"+f) {
			t.Fatalf("buildctl args missing the %s secret: %v", f, buildkit.Args)
		}
	}
	for _, c := range spec.InitContainers {
		mounts := slices.ContainsFunc(c.VolumeMounts, func(m corev1.VolumeMount) bool { return m.Name == managedOIDCVolume })
		if mounts != (c.Name == "prepare-native-build") {
			t.Fatalf("container %s mounts the token Secret = %v; only the preparer may", c.Name, mounts)
		}
		for _, e := range c.Env {
			if strings.Contains(e.Name, "WEB_IDENTITY") || e.Name == "GOOGLE_APPLICATION_CREDENTIALS" {
				t.Fatalf("container %s carries %s in its own environment", c.Name, e.Name)
			}
		}
	}
}

// Render: "Currently, OIDC auth is not available at build time for services
// that build from a Dockerfile." A Dockerfile or buildpack build handed the
// same decision gets no volume, no secret, no variable and no mount, and a
// native build without one is the Job it was before.
func TestOnlyNativeBuildsReadManagedOIDC(t *testing.T) {
	plain := nativeOptions()
	plainJob := BuildJob(plain, plain.ImageRef())
	withOIDC := oidcBuildOptions()
	withOIDC.ManagedOIDCSecret, withOIDC.ManagedOIDC = "", nil
	if got := BuildJob(withOIDC, withOIDC.ImageRef()); !equalJSON(t, got, plainJob) {
		t.Fatal("a native build without a decision changed its Job")
	}
	for _, builder := range []string{BuilderDockerfile, BuilderAuto} {
		o := oidcBuildOptions()
		o.Builder = builder
		job := BuildJob(o, o.ImageRef())
		blob := mustJSON(t, job)
		for _, reject := range []string{managedOIDCVolume, "bld-web-oidc", "bex-oidc-", "/var/lib/render/oidc", "WEB_IDENTITY_TOKEN_FILE", "BEX_NATIVE_OIDC_FILES"} {
			if strings.Contains(blob, reject) {
				t.Fatalf("%s build Job carries %q", builder, reject)
			}
		}
		bare := o
		bare.ManagedOIDCSecret, bare.ManagedOIDC = "", nil
		if !equalJSON(t, job, BuildJob(bare, bare.ImageRef())) {
			t.Fatalf("%s build Job changed by a Managed OIDC decision", builder)
		}
	}
	kp := oidcBuildOptions()
	kp.Builder = BuilderBuildpack
	if blob := mustJSON(t, KpackImage(kp)); strings.Contains(blob, "oidc") || strings.Contains(blob, "WEB_IDENTITY") {
		t.Fatalf("buildpack Image carries Managed OIDC: %s", blob)
	}
}

func TestNativeManagedOIDCValidation(t *testing.T) {
	for name, mutate := range map[string]func(*Options){
		"a Secret without providers": func(o *Options) { o.ManagedOIDC = nil },
		"providers without a Secret": func(o *Options) { o.ManagedOIDCSecret = "" },
		"a variable the loader cannot export": func(o *Options) {
			o.ManagedOIDC = []appv1alpha1.ManagedOIDCProvider{{Name: "x", FileEnv: "X;rm -rf /", File: "x-token"}}
		},
		"a file name the mount cannot carry": func(o *Options) {
			o.ManagedOIDC = []appv1alpha1.ManagedOIDCProvider{{Name: "x", FileEnv: "X_FILE", File: "../x"}}
		},
	} {
		o := oidcBuildOptions()
		mutate(&o)
		if err := validateNativeOptions(o); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
}

// The preparer snapshots each file from the Secret into the build's scratch
// directory byte for byte, which is what buildctl hands BuildKit; a file the
// Secret lacks fails it.
func TestNativePreparerSnapshotsTheManagedOIDCFiles(t *testing.T) {
	o := oidcBuildOptions()
	dir := t.TempDir()
	secret := filepath.Join(dir, "oidc-secret")
	must(t, os.Mkdir(secret, 0o755))
	want := map[string]string{"aws-token": "eyJ.aws.sig", "gcp-token": "eyJ.gcp.sig", "gcp-credentials.json": `{"type":"external_account"}`}
	for name, content := range want {
		must(t, os.WriteFile(filepath.Join(secret, name), []byte(content), 0o644))
	}
	run := func() ([]byte, error) {
		prep := nativeBuildPreparer(o)
		script := strings.NewReplacer(managedOIDCPrepareMount, secret, "/native-files", filepath.Join(dir, "absent"),
			"/native", filepath.Join(dir, "native"), "/runtime-env", filepath.Join(dir, "absent")).Replace(prep.Args[0])
		must(t, os.RemoveAll(filepath.Join(dir, "native")))
		must(t, os.Mkdir(filepath.Join(dir, "native"), 0o755))
		cmd := exec.Command("sh", "-eu", "-c", script)
		cmd.Env = os.Environ()
		for name, value := range expandedBuildValues(prep.Env) {
			cmd.Env = append(cmd.Env, name+"="+value)
		}
		return cmd.CombinedOutput()
	}
	if out, err := run(); err != nil {
		t.Fatalf("prepare: %v: %s", err, out)
	}
	for name, content := range want {
		got, err := os.ReadFile(filepath.Join(dir, "native", "oidc", name))
		if err != nil || string(got) != content {
			t.Fatalf("%s snapshot = %q, %v; want %q", name, got, err, content)
		}
	}
	must(t, os.Remove(filepath.Join(secret, "gcp-token")))
	if out, err := run(); err == nil {
		t.Fatalf("a file missing from the Secret must fail the preparer: %s", out)
	}
}

// TestNativeManagedOIDCPinnedBuildKit is w2/m480 t003's execution gate. It
// runs the preparer and the exact Job buildctl arguments through the pinned
// BuildKit image, then reads the exported image: the authored build command
// reads each token at its runtime path, by its variable, and verifies its
// bytes; a Dockerfile build of the same Options sees neither variable nor
// file; and no blob of the image (config, manifest, any layer) carries a
// token. Opt-in: needs a privileged Docker daemon and pulls the node
// toolchain.
func TestNativeManagedOIDCPinnedBuildKit(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to run against the pinned BuildKit image")
	}
	awsToken := "eyJhbGciOiJFUzI1NiJ9.bex-m480-aws-build-token-marker.c2ln"
	gcpToken := "eyJhbGciOiJFUzI1NiJ9.bex-m480-gcp-build-token-marker.c2ln"
	gcpConfig := `{"type":"external_account","credential_source":{"file":"/var/lib/render/oidc/gcp-token","format":{"type":"text"}}}`
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

	dir := t.TempDir()
	for _, sub := range []string{"src", "oidc-secret", "runtime-env", "native", "out"} {
		must(t, os.Mkdir(filepath.Join(dir, sub), 0o755))
	}
	must(t, os.WriteFile(filepath.Join(dir, "src", "README"), []byte("fixture\n"), 0o644))
	for name, content := range map[string]string{"aws-token": awsToken, "gcp-token": gcpToken, "gcp-credentials.json": gcpConfig} {
		must(t, os.WriteFile(filepath.Join(dir, "oidc-secret", name), []byte(content), 0o644))
	}
	// The service set the platform's variable itself: Render says not to,
	// and the platform's path wins.
	for name, content := range map[string]string{"AWS_ROLE_ARN": "arn:aws:iam::123456789012:role/build", "AWS_WEB_IDENTITY_TOKEN_FILE": "/tmp/mine"} {
		must(t, os.WriteFile(filepath.Join(dir, "runtime-env", name), []byte(content), 0o644))
	}

	o := oidcBuildOptions()
	o.BuildEnv = nil
	o.StartCommand = "cat oidc-verified.txt"
	o.BuildCommand = strings.Join([]string{
		`test "$AWS_WEB_IDENTITY_TOKEN_FILE" = /var/lib/render/oidc/aws-token`,
		`test "$GOOGLE_APPLICATION_CREDENTIALS" = /var/lib/render/oidc/gcp-credentials.json`,
		`printf '%s  %s\n' ` + sum(awsToken) + ` "$AWS_WEB_IDENTITY_TOKEN_FILE" | sha256sum -c`,
		`printf '%s  %s\n' ` + sum(gcpToken) + ` /var/lib/render/oidc/gcp-token | sha256sum -c`,
		`printf '%s  %s\n' ` + sum(gcpConfig) + ` "$GOOGLE_APPLICATION_CREDENTIALS" | sha256sum -c`,
		`test ! -w "$AWS_WEB_IDENTITY_TOKEN_FILE" || [ "$(id -u)" = 0 ]`,
		`echo "verified for $AWS_ROLE_ARN" > oidc-verified.txt`,
	}, " && ")
	if err := validateNativeOptions(o); err != nil {
		t.Fatal(err)
	}
	prep := nativeBuildPreparer(o)
	script := strings.NewReplacer(managedOIDCPrepareMount, filepath.Join(dir, "oidc-secret"), "/native-files", filepath.Join(dir, "absent"),
		"/native", filepath.Join(dir, "native"), "/runtime-env", filepath.Join(dir, "runtime-env")).Replace(prep.Args[0])
	cmd := exec.Command("sh", "-eu", "-c", script)
	cmd.Env = os.Environ()
	for name, value := range expandedBuildValues(prep.Env) {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare: %v: %s", err, out)
	}
	log := runJobBuildctl(t, o, filepath.Join(dir, "src"), filepath.Join(dir, "native"), filepath.Join(dir, "out"))
	for _, token := range []string{awsToken, gcpToken} {
		if strings.Contains(string(log), token) {
			t.Fatalf("build log carries a token:\n%s", log)
		}
	}
	config, layer := lastOCILayer(t, filepath.Join(dir, "out", "image.tar"))
	if got := string(layer["opt/render/project/src/oidc-verified.txt"]); got != "verified for arn:aws:iam::123456789012:role/build\n" {
		t.Fatalf("the build command's verdict = %q; want it to have read and verified every token", got)
	}
	for name := range layer {
		if strings.HasPrefix(name, "var/lib/render/oidc/") {
			t.Fatalf("a token mount was committed into the RUN layer: %s", name)
		}
	}
	for _, marker := range []string{awsToken, gcpToken, "bex-m480-"} {
		if blob := ociBlobContaining(t, filepath.Join(dir, "out", "image.tar"), marker); blob != "" {
			t.Fatalf("image blob %s carries %q", blob, marker)
		}
	}
	// The variables are the build's alone: the image's environment, which
	// every container started from it inherits, names no token file.
	var image struct{ Config struct{ Env []string } }
	if err := json.Unmarshal(config, &image); err != nil {
		t.Fatal(err)
	}
	for _, e := range image.Config.Env {
		if strings.HasPrefix(e, "AWS_") || strings.HasPrefix(e, "GOOGLE_APPLICATION_CREDENTIALS=") {
			t.Fatalf("image environment keeps %s", e)
		}
	}

	// The same Options as a Dockerfile build: BuildKit runs a RUN that would
	// see the variable or the file, and both are absent.
	docker := o
	docker.Builder = BuilderDockerfile
	docker.RuntimeEnvSecret = ""
	ddir := t.TempDir()
	for _, sub := range []string{"src", "native", "out"} {
		must(t, os.Mkdir(filepath.Join(ddir, sub), 0o755))
	}
	must(t, os.WriteFile(filepath.Join(ddir, "src", "Dockerfile"), []byte("FROM "+nativeToolchains["node"].defaultImage()+"\n"+
		`RUN test -z "${AWS_WEB_IDENTITY_TOKEN_FILE:-}${GOOGLE_APPLICATION_CREDENTIALS:-}" && test ! -e /var/lib/render/oidc && echo none > /oidc-none`+"\n"), 0o644))
	dlog := runJobBuildctl(t, docker, filepath.Join(ddir, "src"), filepath.Join(ddir, "native"), filepath.Join(ddir, "out"),
		"-v", filepath.Join(ddir, "src")+":"+sourceMount+":ro") // the Job reads its Dockerfile from the checkout

	if _, layer := lastOCILayer(t, filepath.Join(ddir, "out", "image.tar")); string(layer["oidc-none"]) != "none\n" {
		t.Fatalf("the Dockerfile build saw a token or its variable:\n%s", dlog)
	}
}

// ociBlobContaining is the first blob of the OCI archive (its config, a
// manifest, or any layer, decompressed) whose bytes contain marker; "" when
// none does.
func ociBlobContaining(t *testing.T, archive, marker string) string {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte(marker)) {
			return hdr.Name
		}
		if gz, err := gzip.NewReader(bytes.NewReader(raw)); err == nil {
			if plain, err := io.ReadAll(gz); err == nil && bytes.Contains(plain, []byte(marker)) {
				return fmt.Sprintf("%s (decompressed)", hdr.Name)
			}
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func equalJSON(t *testing.T, a, b any) bool {
	t.Helper()
	return mustJSON(t, a) == mustJSON(t, b)
}
