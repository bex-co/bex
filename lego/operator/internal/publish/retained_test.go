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

package publish

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// retainedOptions restores mysite's rev-1 publication into rev-4 (w4/m197).
func retainedOptions() Options {
	o := testOptions()
	o.Image = ""
	o.PublishPath = "a-directory-that-no-longer-exists"
	o.SourcePrefix = "mysite/rev-1/"
	o.Revision = "rev-4"
	return o
}

func TestRetainedCopyJobNeverReadsTheImageOrPublishPath(t *testing.T) {
	o := retainedOptions()
	o.PullSecret = "bex-registry-pull"
	job := PublishJob(o)
	pod := job.Spec.Template.Spec
	if len(pod.InitContainers) != 1 || pod.InitContainers[0].Name != "fetch" {
		t.Fatalf("init containers = %+v, want one fetch step", pod.InitContainers)
	}
	fetch := pod.InitContainers[0]
	if fetch.Image != DefaultAWSCLIImage {
		t.Fatalf("fetch image = %q, want the pinned uploader image", fetch.Image)
	}
	env := map[string]string{}
	for _, e := range fetch.Env {
		env[e.Name] = e.Value
	}
	if env["SOURCE_URI"] != "s3://bex-static/mysite/rev-1/" || env["ENDPOINT"] != o.Store.Endpoint {
		t.Fatalf("fetch env = %v, want the retained rev-1 prefix on the configured store", env)
	}
	if strings.Contains(strings.Join(fetch.Command, " "), o.PublishPath) || env["PUBLISH_PATH"] != "" {
		t.Fatal("a retained copy must not consult the saved publish directory")
	}
	if len(fetch.EnvFrom) != 1 || fetch.EnvFrom[0].SecretRef.Name != o.Store.Secret {
		t.Fatalf("fetch envFrom = %+v, want the publish credential", fetch.EnvFrom)
	}
	if fetch.TerminationMessagePolicy == corev1.TerminationMessageFallbackToLogsOnError {
		t.Fatal("fetch holds object-store credentials and must not echo its logs")
	}
	if len(pod.ImagePullSecrets) != 0 {
		t.Fatal("a retained copy pulls only the public uploader image")
	}
	upload := pod.Containers[0]
	if got := strings.Join(upload.Args, " "); !strings.Contains(got, "s3://bex-static/mysite/rev-4/") {
		t.Fatalf("upload args = %q, want the new rev-4 prefix", got)
	}
	if job.Name != JobName("mysite", "rev-4") {
		t.Fatalf("job name = %q, want the new revision's publish Job", job.Name)
	}
}

func TestRetainedCopyValidation(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		mutate func(*Options)
		want   string
	}{
		"image and source":        {func(o *Options) { o.Image = "img" }, "exactly one"},
		"repo and source":         {func(o *Options) { o.Repo = "https://github.com/x/y" }, "exactly one"},
		"another app's prefix":    {func(o *Options) { o.SourcePrefix = "othersite/rev-1/" }, "not one of this App's revisions"},
		"other workspace":         {func(o *Options) { o.Workspace = "tea-a"; o.SourcePrefix = "tea-b/mysite/rev-1/" }, "not one of this App's revisions"},
		"path escape":             {func(o *Options) { o.SourcePrefix = "mysite/rev-1/../../x/" }, "not one of this App's revisions"},
		"the revision being made": {func(o *Options) { o.SourcePrefix = "mysite/rev-4/" }, "the revision being published"},
	} {
		t.Run(name, func(t *testing.T) {
			o := retainedOptions()
			tc.mutate(&o)
			o.Client = fake.NewClientBuilder().Build()
			if _, err := Ensure(ctx, o); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Ensure = %v, want %q", err, tc.want)
			}
		})
	}
	for name, mutate := range map[string]func(*Options){
		"own legacy root":  func(o *Options) { o.Workspace = "tea-a" },
		"own scoped root":  func(o *Options) { o.Workspace = "tea-a"; o.SourcePrefix = "tea-a/mysite/rev-1/" },
		"empty publishDir": func(o *Options) { o.PublishPath = "" },
	} {
		t.Run(name, func(t *testing.T) {
			o := retainedOptions()
			mutate(&o)
			o.Client = fake.NewClientBuilder().Build()
			if _, err := Ensure(ctx, o); err != nil {
				t.Fatalf("Ensure = %v, want an accepted retained copy", err)
			}
		})
	}
}

// runFetch executes the fetch container's own script against a fake aws CLI
// that serves the object store from a local directory — the actual consumer,
// not the generated options. It returns the staged files, the termination
// message, and the exit code.
func runFetch(t *testing.T, store map[string]string, awsFails bool) ([]string, string, int) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	out, bin, bucket := filepath.Join(dir, "out"), filepath.Join(dir, "bin"), filepath.Join(dir, "bucket")
	for _, d := range []string{out, bin, bucket} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for key, body := range store {
		p := filepath.Join(bucket, key)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// aws s3 sync s3://<bucket>/<prefix> <dest> ...: copy the prefix's objects;
	// a missing prefix syncs nothing, exactly like S3.
	awsStub := "#!/bin/sh\n"
	if awsFails {
		awsStub += "echo 'AccessDenied' >&2; exit 1\n"
	} else {
		awsStub += `src="${3#s3://bex-static/}"; [ -d "$BUCKET/$src" ] && cp -R "$BUCKET/$src". "$4"/; exit 0` + "\n"
	}
	if err := os.WriteFile(filepath.Join(bin, "aws"), []byte(awsStub), 0o755); err != nil {
		t.Fatal(err)
	}
	fetch := PublishJob(retainedOptions()).Spec.Template.Spec.InitContainers[0]
	term := filepath.Join(dir, "termination-log")
	script := strings.ReplaceAll(fetch.Command[2], outMount, out)
	script = strings.ReplaceAll(script, "/dev/termination-log", term)
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "BUCKET=" + bucket}
	for _, e := range fetch.Env {
		cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
	}
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	entries, _ := os.ReadDir(out)
	staged := make([]string, 0, len(entries))
	for _, e := range entries {
		staged = append(staged, e.Name())
	}
	msg, _ := os.ReadFile(term)
	return staged, strings.TrimSpace(string(msg)), code
}

func TestRetainedFetchStagesTheTargetsPublishedFiles(t *testing.T) {
	staged, msg, code := runFetch(t, map[string]string{
		"mysite/rev-1/index.html": "qa-a79-v1\n",
		"mysite/rev-3/index.html": "qa-a79-v2\n",
	}, false)
	if code != 0 || msg != "" || len(staged) != 1 || staged[0] != "index.html" {
		t.Fatalf("fetch = staged %v, msg %q, exit %d; want rev-1's index.html", staged, msg, code)
	}
}

// Restoring nothing must fail in its own words; an empty upload would
// otherwise publish a blank site and report it Live.
func TestRetainedFetchRefusesAnEmptyOrMissingSource(t *testing.T) {
	staged, msg, code := runFetch(t, map[string]string{"mysite/rev-3/index.html": "v2"}, false)
	if code == 0 || len(staged) != 0 || !strings.Contains(msg, "mysite/rev-1/ is empty or no longer available") {
		t.Fatalf("fetch = staged %v, msg %q, exit %d; want a named empty-source failure", staged, msg, code)
	}
}

func TestRetainedFetchStoreFailureIsAPlatformFault(t *testing.T) {
	_, msg, code := runFetch(t, map[string]string{"mysite/rev-1/index.html": "v1"}, true)
	if code == 0 || !strings.Contains(msg, "could not be read from the static-site store") || strings.Contains(msg, "AccessDenied") {
		t.Fatalf("fetch = msg %q, exit %d; want the fixed platform-fault line, never aws-cli's output", msg, code)
	}
}
