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

package main_test

// `bex s3` against the built launcher (ADR112 D3, ADR113 D7): a fake `aws` on
// PATH and a fake bex-api. The fake `aws` records what it was run with, so the
// tests read the command line and the environment the real AWS CLI would get,
// and prove the launcher replaced itself with it.

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	s3Workspace = "tea-bex"
	s3Endpoint  = "https://objects.example.test"
	s3KeyID     = "s3k-d0abc123def456ghij78"
	s3Secret    = "fake-secret-of-the-launch-contract"
	// theirSecret is a credential of the person's own AWS setup, which must
	// never reach the child.
	theirSecret = "THEIR-OWN-AWS-SECRET"
)

// s3MintRoutes is the key-mint answer bex-api gives: an ephemeral key good for
// twelve hours from the real clock, which the launcher reads.
func s3MintRoutes() map[string]string {
	expires := time.Now().Add(12 * time.Hour).UTC().Format(time.RFC3339)
	return map[string]string{
		"/v1/object-storage/" + s3Workspace + "/access-keys": `{"id":"` + s3KeyID + `","name":"bex s3","permission":"readwrite",` +
			`"createdBy":"usr-1","createdAt":"2026-11-20T10:00:00Z","expiresAt":"` + expires + `","lastUsedAt":null,"status":"active","secret":"` + s3Secret + `"}`,
	}
}

// fakeAWS puts an `aws` on a fresh PATH entry that writes one argument per
// line to args and its environment to env, then exits 0.
func fakeAWS(t *testing.T) (pathEntry, argsFile, envFile string) {
	t.Helper()
	pathEntry = t.TempDir()
	out := t.TempDir()
	argsFile, envFile = filepath.Join(out, "args"), filepath.Join(out, "env")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\nenv > " + envFile + "\n"
	if err := os.WriteFile(filepath.Join(pathEntry, "aws"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return pathEntry, argsFile, envFile
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("bex did not replace itself with the AWS CLI: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// s3Env is launchEnv for `bex s3`: the active workspace and the endpoint of
// the test control plane, which production's constant does not cover.
func s3Env(t *testing.T, home, apiURL, pathEntry string, extra ...string) []string {
	t.Helper()
	return append(launchEnv(t, home, apiURL, pathEntry), append([]string{"BEX_WORKSPACE=" + s3Workspace, "BEX_S3_ENDPOINT=" + s3Endpoint}, extra...)...)
}

// TestBexS3LaunchEmitsExactlyOneEvent: `bex s3` and `bex s3api` replace bex
// with the AWS CLI, so upstream's hook never runs; each launch reports itself
// once, as `bex s3` or `bex s3api`, without the buckets, keys or arguments.
func TestBexS3LaunchEmitsExactlyOneEvent(t *testing.T) {
	t.Parallel()
	for _, run := range []struct {
		name    string
		args    []string
		command string
		argv    []string
	}{
		{"s3", []string{"s3", "cp", "./launch-contract-file", "s3://launch-contract-bucket/key"}, "bex s3",
			[]string{"s3", "cp", "./launch-contract-file", "s3://launch-contract-bucket/key", "--endpoint-url", s3Endpoint}},
		{"s3api", []string{"s3api", "list-objects-v2", "--bucket", "launch-contract-bucket"}, "bex s3api",
			[]string{"s3api", "list-objects-v2", "--bucket", "launch-contract-bucket", "--endpoint-url", s3Endpoint}},
	} {
		t.Run(run.name, func(t *testing.T) {
			t.Parallel()
			stub := &telemetryStub{routes: s3MintRoutes()}
			api := httptest.NewServer(stub.handler())
			t.Cleanup(api.Close)
			home := t.TempDir()
			seedNoticeMarker(t, home)
			bin, argsFile, _ := fakeAWS(t)

			command := exec.Command(buildBex(), run.args...)
			command.Env = s3Env(t, home, api.URL, bin)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("bex %v: %v\n%s", run.args, err, output)
			}
			if got := lines(t, argsFile); !slices.Equal(got, run.argv) {
				t.Errorf("aws ran with %q, want %q", got, run.argv)
			}
			assertOneLaunchEvent(t, stub, run.command, "launch-contract-bucket", "launch-contract-file", s3KeyID, s3Secret)
		})
	}
}

// TestBexS3GivesTheAWSCLIOnlyBexsKey: whatever AWS setup the person has, the
// AWS CLI bex starts sees bex's endpoint, key and the files bex owns, keeps
// the key out of ~/.aws and caches it owner-only.
func TestBexS3GivesTheAWSCLIOnlyBexsKey(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	seedNoticeMarker(t, home)
	bin, _, envFile := fakeAWS(t)

	command := exec.Command(buildBex(), "s3", "ls")
	command.Env = s3Env(t, home, api.URL, bin,
		"AWS_PROFILE=personal", "AWS_SESSION_TOKEN=their-token", "AWS_ACCESS_KEY_ID=AKIATHEIRS", "AWS_SECRET_ACCESS_KEY="+theirSecret,
		"AWS_CONFIG_FILE="+filepath.Join(home, ".aws", "config"), "AWS_ENDPOINT_URL_S3=https://elsewhere.example",
		"AWS_REGION=eu-west-1", "AWS_PAGER=")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex s3 ls: %v\n%s", err, output)
	}

	seen := map[string]string{}
	for _, item := range lines(t, envFile) {
		if name, value, ok := strings.Cut(item, "="); ok {
			seen[name] = value
		}
	}
	state := filepath.Join(home, ".bex", "state", "s3")
	for name, want := range map[string]string{
		"AWS_ACCESS_KEY_ID": s3KeyID, "AWS_SECRET_ACCESS_KEY": s3Secret, "AWS_REGION": "oregon",
		"AWS_CONFIG_FILE": filepath.Join(state, "aws-config"), "AWS_SHARED_CREDENTIALS_FILE": filepath.Join(state, "aws-credentials"),
	} {
		if seen[name] != want {
			t.Errorf("the AWS CLI saw %s=%q, want %q", name, seen[name], want)
		}
	}
	for _, name := range []string{"AWS_PROFILE", "AWS_SESSION_TOKEN", "AWS_ENDPOINT_URL_S3"} {
		if value, found := seen[name]; found {
			t.Errorf("the AWS CLI saw %s=%q from the person's setup", name, value)
		}
	}
	if _, kept := seen["AWS_PAGER"]; !kept {
		t.Error("AWS_PAGER, a behavior setting, was dropped")
	}
	// Nothing was written into the person's AWS configuration, and the key is
	// cached owner-only where bex keeps its state.
	if _, err := os.Stat(filepath.Join(home, ".aws")); err == nil {
		t.Error("bex created ~/.aws")
	}
	cached, _ := filepath.Glob(filepath.Join(state, "key-*.json"))
	if len(cached) != 1 {
		t.Fatalf("cached keys = %v, want one", cached)
	}
	if info, _ := os.Stat(cached[0]); info.Mode().Perm() != 0o600 {
		t.Errorf("cached key mode = %v, want 0600", info.Mode().Perm())
	}
	if creds, err := os.ReadFile(filepath.Join(state, "aws-credentials")); err != nil || len(creds) != 0 {
		t.Errorf("bex's AWS credentials file = %q, %v; want it empty", creds, err)
	}
}

// pathWithout is the ambient PATH minus every directory that holds a
// `name`: GitHub's runner images ship AWS CLI v2 in /usr/local/bin, so a test
// of "no aws" cannot lean on the host not having one.
func pathWithout(name string) string {
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			kept = append(kept, dir)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

// TestBexS3WithoutAWSIsReportedOnceByUpstream is the no-double-count half:
// with no `aws` the process is never replaced, so Cobra finishes normally,
// upstream's hook reports it, and no key was minted for nothing.
func TestBexS3WithoutAWSIsReportedOnceByUpstream(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	seedNoticeMarker(t, home)

	command := exec.Command(buildBex(), "s3", "ls")
	// The last PATH wins: no aws anywhere, whatever the host installed.
	command.Env = append(s3Env(t, home, api.URL, t.TempDir()), "PATH="+pathWithout("aws"))
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("bex s3 ls succeeded with no aws on PATH:\n%s", output)
	}
	if !strings.Contains(string(output), "bex s3 env") || !strings.Contains(string(output), "AWS CLI") {
		t.Errorf("the refusal does not say how to install aws or what to use instead:\n%s", output)
	}
	event := awaitEvents(t, stub, 1)[0]
	if event["command"] != "bex s3" || event["completion_kind"] != "execution_error" {
		t.Errorf("event = %v, want one execution_error for bex s3", event)
	}
	if cached, _ := filepath.Glob(filepath.Join(home, ".bex", "state", "s3", "key-*.json")); len(cached) != 0 {
		t.Errorf("a key was minted for a run that could not start aws: %v", cached)
	}
}

// TestBexS3NativeCommandsReportOncePerCommand: env, keys and info return
// through Cobra, so upstream's hook reports each once, named by its commands.
func TestBexS3NativeCommandsReportOncePerCommand(t *testing.T) {
	t.Parallel()
	for _, run := range []struct {
		args    []string
		command string
		routes  map[string]string
	}{
		{[]string{"s3", "env"}, "bex s3 env", s3MintRoutes()},
		{[]string{"s3", "keys", "list"}, "bex s3 keys list", map[string]string{"/v1/object-storage/" + s3Workspace + "/access-keys": `[]`}},
		{[]string{"s3", "info", "photos"}, "bex s3 info", map[string]string{"/v1/object-storage/" + s3Workspace + "/buckets/photos": `{"id":"obk-1","name":"photos","region":"oregon","public":false,"objects":0,"bytes":0,"observedAt":null,"createdAt":"2026-11-20T10:00:00Z"}`}},
	} {
		t.Run(run.command, func(t *testing.T) {
			t.Parallel()
			stub := &telemetryStub{routes: run.routes}
			api := httptest.NewServer(stub.handler())
			t.Cleanup(api.Close)
			home := t.TempDir()
			seedNoticeMarker(t, home)

			command := exec.Command(buildBex(), run.args...)
			command.Env = s3Env(t, home, api.URL, os.Getenv("PATH"))
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("bex %v: %v\n%s", run.args, err, output)
			}
			assertOneLaunchEvent(t, stub, run.command, "photos", s3KeyID, s3Secret)
		})
	}
}

// TestGateDoesNotReachTheAWSCLI: the disclosure gate denies consent to the
// sender upstream builds, not to the AWS CLI bex starts, which must not
// inherit an opt-out the person never set.
func TestGateDoesNotReachTheAWSCLI(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	bin, _, envFile := fakeAWS(t)

	command := exec.Command(buildBex(), "s3", "ls")
	command.Env = s3Env(t, t.TempDir(), api.URL, bin)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex s3 ls: %v\n%s", err, output)
	}
	for _, item := range lines(t, envFile) {
		if strings.HasPrefix(item, "RENDER_CLI_DISABLE_ANALYTICS=") {
			t.Errorf("the AWS CLI inherited %s from the disclosure gate", item)
		}
	}
}
