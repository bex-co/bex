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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestValkeyRuntimeTLSSwitch runs the pod's own start command on both pinned
// Valkey images (BEX_TEST_VALKEY=1) and drives the operator's real switch
// against it (w8/m134 t003):
//   - started without the certificate, no TLS listener runs;
//   - an enable before the kubelet projected the files is refused and leaves
//     the listener closed and the keyspace intact;
//   - with the files present, the enable opens the listener in the same
//     process (same run_id) and a Persistence Off key survives;
//   - tls-port 0 closes it, again in the same process;
//   - a restart with the files mounted starts listening on its own, and one
//     without them does not.
func TestValkeyRuntimeTLSSwitch(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker TLS switch tests")
	}
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) { testValkeyRuntimeTLSSwitch(t, major) })
	}
}

// tlsSwitchEngine is one pinned Valkey container started with the pod's own
// start command, its certificate directory on a separate volume standing in
// for the optional Secret mount.
type tlsSwitchEngine struct {
	*persistenceEngine
	ctx              context.Context
	id               string
	tlsAddr          string
	tenant, platform *redis.Client
	roots            *x509.CertPool
}

func startTLSSwitchEngine(t *testing.T, major string) *tlsSwitchEngine {
	e := &tlsSwitchEngine{persistenceEngine: newPersistenceEngine(t, major), ctx: context.Background()}
	certVolume := strings.TrimSpace(e.docker("volume", "create"))
	t.Cleanup(func() { e.docker("volume", "rm", certVolume) })
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "tls-engine", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", Version: major, PersistenceMode: appv1alpha1.KeyValuePersistenceOff},
	}
	plan, storage := resolveKVPlan(kv.Spec)
	sts := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false, true))
	server := sts.Spec.Template.Spec.Containers[0]
	args := make([]string, 0, 13+len(server.Args))
	args = append(args, "run", "-d", "--user", "999:1000", "-p", "127.0.0.1::6379", "-p", "127.0.0.1::6380",
		"-v", e.volume+":/data", "-v", certVolume+":"+kvTLSMountPath, server.Image)
	for _, arg := range server.Args {
		arg = strings.ReplaceAll(arg, "$("+kvPasswordEnv+")", fixturePassword)
		args = append(args, strings.ReplaceAll(arg, "$("+kvPlatformPasswordEnv+")", fixturePlatformPassword))
	}
	out := strings.TrimSpace(e.docker(args...))
	e.id = out[strings.LastIndex(out, "\n")+1:]
	t.Cleanup(func() {
		e.closeClients()
		e.docker("rm", "-f", e.id)
	})
	e.waitServing()
	return e
}

func (e *tlsSwitchEngine) closeClients() {
	if e.tenant != nil {
		_ = e.tenant.Close()
		_ = e.platform.Close()
	}
}

func (e *tlsSwitchEngine) hostPort(port string) string {
	out := strings.TrimSpace(e.docker("port", e.id, port+"/tcp"))
	return strings.Split(out, "\n")[0]
}

// waitServing reconnects (docker restart re-publishes the ports on new host
// ports) and waits for an authenticated PING.
func (e *tlsSwitchEngine) waitServing() {
	e.t.Helper()
	e.closeClients()
	plainAddr := e.hostPort("6379")
	e.tlsAddr = e.hostPort("6380")
	e.tenant = redis.NewClient(&redis.Options{Addr: plainAddr, Password: fixturePassword, Protocol: 2, MaxRetries: -1})
	e.platform = platformValkeyClient(plainAddr, &corev1.Secret{Data: map[string][]byte{kvPasswordKey: []byte(fixturePlatformPassword)}})
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if e.tenant.Ping(e.ctx).Err() == nil {
			return
		}
	}
	e.t.Fatalf("Valkey did not start:\n%s", e.docker("logs", e.id))
}

func (e *tlsSwitchEngine) restart() {
	e.t.Helper()
	e.docker("restart", "-t", "10", e.id)
	e.waitServing()
}

func (e *tlsSwitchEngine) runID() string {
	e.t.Helper()
	info, err := e.platform.Info(e.ctx, "server").Result()
	if err != nil {
		e.t.Fatal(err)
	}
	for line := range strings.SplitSeq(info, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "run_id:"); ok {
			return v
		}
	}
	e.t.Fatal("no run_id")
	return ""
}

// expectTLSPort asserts the process's tls-port and that a TLS client gets an
// authenticated PING exactly when it is open.
func (e *tlsSwitchEngine) expectTLSPort(step, want string) {
	e.t.Helper()
	got, err := e.platform.ConfigGet(e.ctx, "tls-port").Result()
	if err != nil {
		e.t.Fatal(err)
	}
	if got["tls-port"] != want {
		e.t.Fatalf("%s: tls-port = %q, want %q", step, got["tls-port"], want)
	}
	c := redis.NewClient(&redis.Options{Addr: e.tlsAddr, Password: fixturePassword, Protocol: 2, MaxRetries: -1,
		DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second,
		TLSConfig: &tls.Config{RootCAs: e.roots, ServerName: "tls-engine.kv.example.test", MinVersion: tls.VersionTLS12}})
	defer func() { _ = c.Close() }()
	if err := c.Ping(e.ctx).Err(); (err == nil) != (want != "0") {
		e.t.Fatalf("%s: TLS PING = %v with tls-port %s", step, err, want)
	}
}

func (e *tlsSwitchEngine) expectKey(step string) {
	e.t.Helper()
	if got, err := e.tenant.Get(e.ctx, "kept").Result(); err != nil || got != "yes" {
		e.t.Fatalf("%s lost the keyspace: %q, %v", step, got, err)
	}
}

// switchTo runs the operator's switch and asserts its outcome.
func (e *tlsSwitchEngine) switchTo(step string, on, wantListening, wantErr bool) {
	e.t.Helper()
	listening, err := switchValkeyTLS(e.ctx, e.platform, on)
	if listening != wantListening || (err != nil) != wantErr {
		e.t.Fatalf("%s: listening %v, err %v\n%s", step, listening, err, e.docker("logs", e.id))
	}
}

// projectCertificate stands in for the kubelet writing the Secret's files.
func (e *tlsSwitchEngine) projectCertificate() {
	e.t.Helper()
	certPEM, keyPEM := selfSignedValkeyCertificate(e.t)
	e.roots = x509.NewCertPool()
	e.roots.AppendCertsFromPEM(certPEM)
	dir := e.t.TempDir()
	for name, data := range map[string][]byte{corev1.TLSCertKey: certPEM, corev1.TLSPrivateKeyKey: keyPEM} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			e.t.Fatal(err)
		}
		e.docker("cp", path, e.id+":"+kvTLSMountPath+"/"+name)
	}
}

func testValkeyRuntimeTLSSwitch(t *testing.T, major string) {
	e := startTLSSwitchEngine(t, major)
	e.expectTLSPort("started without a certificate", "0")
	if err := e.tenant.Set(e.ctx, "kept", "yes", 0).Err(); err != nil {
		t.Fatal(err)
	}
	birth := e.runID()

	// The Secret exists, but the kubelet has not projected it yet.
	e.switchTo("enable without the files", true, false, true)
	e.expectTLSPort("a refused enable", "0")
	e.expectKey("a refused enable")

	e.projectCertificate()
	e.switchTo("enable", true, true, false)
	e.expectTLSPort("enabled", "6380")
	e.expectKey("the enable")
	e.switchTo("a repeated enable", true, true, false)
	if e.runID() != birth {
		t.Fatal("the enable restarted the process")
	}

	e.switchTo("disable", false, false, false)
	e.expectTLSPort("disabled", "0")
	e.expectKey("the disable")
	if e.runID() != birth {
		t.Fatal("the disable restarted the process")
	}

	// A restart with the Secret mounted comes back listening by itself.
	e.restart()
	if e.runID() == birth {
		t.Fatal("docker restart did not restart Valkey")
	}
	e.expectTLSPort("restarted with the certificate mounted", "6380")

	// And without it, it starts without the listener.
	e.docker("exec", "-u", "0", e.id, "rm", kvTLSCertFile, kvTLSKeyFile)
	e.restart()
	e.expectTLSPort("restarted without the certificate", "0")
}

// selfSignedValkeyCertificate stands in for cert-manager's issuance. It is its
// own CA because the listener names tls.crt as tls-ca-cert-file too.
func selfSignedValkeyCertificate(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "tls-engine.kv.example.test"},
		DNSNames: []string{"tls-engine.kv.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
