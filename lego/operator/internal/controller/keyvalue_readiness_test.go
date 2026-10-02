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

package controller

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestValkeyReadinessProbeIsAuthenticatedPing pins w1/m166 t002: readiness is
// an authenticated PING through the projected pod spec, never a bare socket
// accept, and the password reaches valkey-cli through the environment only.
func TestValkeyReadinessProbeIsAuthenticatedPing(t *testing.T) {
	for _, public := range []bool{false, true} {
		var spec corev1.PodSpec
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-x", Namespace: "ws"}}
		plan, _ := resolveKVPlan(kv.Spec)
		applyValkeyPodSpec(&spec, kv, keyValueIntent{plan: plan, public: public, authSecretName: "red-x-auth", tlsSecretName: "red-x-kv-tls"})
		valkey := spec.Containers[0]
		probe := valkey.ReadinessProbe
		if probe == nil || probe.TCPSocket != nil || probe.Exec == nil {
			t.Fatalf("public=%v: readiness probe = %+v, want an exec probe and no TCP socket check", public, probe)
		}
		script := strings.Join(probe.Exec.Command, " ")
		if !strings.Contains(script, `REDISCLI_AUTH="$VALKEY_PASSWORD"`) || strings.Contains(script, " -a ") {
			t.Fatalf("public=%v: probe must authenticate via REDISCLI_AUTH, never argv: %q", public, script)
		}
		if valkey.Env[0].Name != "VALKEY_PASSWORD" || valkey.Env[0].ValueFrom.SecretKeyRef.Name != "red-x-auth" {
			t.Fatalf("public=%v: probe's VALKEY_PASSWORD is not the auth Secret: %+v", public, valkey.Env)
		}
	}
}

// TestValkeyReadinessProbeScript runs the projected probe command against a
// stand-in valkey-cli: only an authenticated PONG passes. A TCP probe passed
// every one of these failure cases (the socket accepts throughout).
func TestValkeyReadinessProbeScript(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	command := valkeyReadinessProbe().Exec.Command
	if command[0] != "sh" || command[1] != "-c" {
		t.Fatalf("probe command = %q, want sh -c", command)
	}
	for _, tc := range []struct {
		name, reply string
		want        bool
	}{
		{"authenticated PONG", "PONG", true},
		{"dataset still loading", "LOADING Valkey is loading the dataset in memory", false},
		{"wrong password", "AUTH failed: WRONGPASS invalid username-password pair or user is disabled.", false},
		{"auth required", "NOAUTH Authentication required.", false},
		{"server unreachable", "Could not connect to Valkey at 127.0.0.1:6379: Connection refused", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			// The stand-in answers PONG only when it was handed the password
			// through REDISCLI_AUTH and asked to PING on the plaintext port.
			fake := "#!/bin/sh\n" +
				`if [ "$REDISCLI_AUTH" != "s3cret" ]; then echo "NOAUTH Authentication required."; exit 0; fi` + "\n" +
				`if [ "$1 $2 $3" != "-p 6379 PING" ]; then echo "bad args: $*"; exit 0; fi` + "\n" +
				"echo '" + tc.reply + "'\n"
			if err := os.WriteFile(filepath.Join(bin, "valkey-cli"), []byte(fake), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(sh, command[1:]...)
			cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "VALKEY_PASSWORD=s3cret"}
			if got := cmd.Run() == nil; got != tc.want {
				t.Fatalf("probe passed=%v for reply %q, want %v", got, tc.reply, tc.want)
			}
		})
	}
}
