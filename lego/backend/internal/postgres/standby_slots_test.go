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

package postgres

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The standby slot sweep of w34/008. Which instances it reaches and how it
// authenticates are checked here; what it drops, on real PostgreSQL, in
// standby_slots_pg_test.go.

// testCA signs the certificates of these tests.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

var testSerial int64

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// sign issues tmpl under parent (self-signed when parent is nil).
func sign(t *testing.T, tmpl *x509.Certificate, parent *testCA, key *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	testSerial++
	tmpl.SerialNumber = big.NewInt(testSerial)
	tmpl.NotBefore, tmpl.NotAfter = time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	issuer, signer := tmpl, key
	if parent != nil {
		issuer, signer = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, &key.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func newTestCA(t *testing.T, name string, parent *testCA) *testCA {
	t.Helper()
	key := newTestKey(t)
	cert := sign(t, &x509.Certificate{Subject: pkix.Name{CommonName: name}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}, parent, key)
	return &testCA{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})}
}

// leaf issues an end-entity certificate for usage.
func (ca *testCA) leaf(t *testing.T, cn string, usage x509.ExtKeyUsage) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key := newTestKey(t)
	return sign(t, &x509.Certificate{Subject: pkix.Name{CommonName: cn}, DNSNames: []string{cn},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}, ca, key), key
}

// replicationSecret is CNPG's "<cluster>-replication": the streaming_replica
// client certificate its CA signed.
func replicationSecret(t *testing.T, cluster string, ca *testCA) *corev1.Secret {
	t.Helper()
	cert, key := ca.leaf(t, cnpgStreamingReplicaUser, x509.ExtKeyUsageClientAuth)
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cluster + cnpgReplicationCertSuffix, Namespace: "default"},
		Type: corev1.SecretTypeTLS, Data: map[string][]byte{
			corev1.TLSCertKey:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}),
			corev1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		}}
}

func caSecret(name string, ca *testCA) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Data: map[string][]byte{"ca.crt": ca.pem}}
}

// instancePod is a CNPG instance Pod of cluster in role, Running at ip ("" for
// one not scheduled yet).
func instancePod(name, cluster, role, ip string) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{
		core.PodLabelCNPGCluster: cluster, cnpgInstanceRoleLabel: role,
	}}}
	if ip != "" {
		pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, PodIP: ip}
	} else {
		pod.Status.Phase = corev1.PodPending
	}
	return pod
}

func haPublisher(name string) *appv1alpha1.Database {
	d := publisherDatabase(name)
	d.Spec.HighAvailability = true
	d.Spec.LogicalReplication = &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}}
	return d
}

// Only an HA publisher's own standbys are reached: not its primary, not a
// read replica (a Cluster of its own, which a user may decode from), not an
// instance without an address, and nothing of a database whose standby does
// not synchronize slots. Each is reached as streaming_replica in the postgres
// database, at its Pod IP, with the replication client certificate and no
// plaintext fallback.
func TestStandbySlotSweepReachesOnlyAnHAPublishersStandby(t *testing.T) {
	ca := newTestCA(t, "dpg-ha-pub-ca", nil)
	pub := haPublisher("dpg-ha-pub")
	pub.Spec.ReadReplicas = []appv1alpha1.DatabaseReadReplica{{Name: "r1"}}
	plain := haPublisher("dpg-ha-plain")
	plain.Spec.LogicalReplication = nil
	single := haPublisher("dpg-single")
	single.Spec.HighAvailability = false
	asleep := haPublisher("dpg-ha-asleep")
	asleep.Spec.Suspended = true
	objs := []client.Object{pub, plain, single, asleep,
		replicationSecret(t, pub.Name, ca), caSecret(pub.Name+"-ca", ca),
		instancePod("dpg-ha-pub-1", pub.Name, "primary", "10.0.0.1"),
		instancePod("dpg-ha-pub-2", pub.Name, cnpgReplicaInstanceRole, "10.0.0.2"),
		instancePod("dpg-ha-pub-3", pub.Name, cnpgReplicaInstanceRole, ""),
		instancePod("dpg-ha-pub-ro-r1-1", appv1alpha1.ReadReplicaClusterName(pub.Name, "r1"), cnpgReplicaInstanceRole, "10.0.0.9"),
	}
	for i, d := range []*appv1alpha1.Database{plain, single, asleep} {
		ip := fmt.Sprintf("10.0.1.%d", i+1)
		objs = append(objs, replicationSecret(t, d.Name, ca), caSecret(d.Name+"-ca", ca),
			instancePod(d.Name+"-2", d.Name, cnpgReplicaInstanceRole, ip))
	}
	svc, _ := newService(objs...)
	var reached []*pgx.ConnConfig
	w := &StandbySlotSweeper{Service: svc, sweepStandby: func(_ context.Context, cfg *pgx.ConnConfig) ([]string, error) {
		reached = append(reached, cfg)
		return []string{"s_ha"}, nil
	}}
	if err := w.SweepOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(reached) != 1 {
		hosts := make([]string, len(reached))
		for i, cfg := range reached {
			hosts[i] = cfg.Host
		}
		t.Fatalf("reached %v, want only the HA publisher's standby 10.0.0.2", hosts)
	}
	cfg := reached[0]
	if cfg.Host != "10.0.0.2" || cfg.Port != 5432 || cfg.Database != "postgres" || cfg.User != cnpgStreamingReplicaUser {
		t.Fatalf("dialed %s@%s:%d/%s, want streaming_replica@10.0.0.2:5432/postgres", cfg.User, cfg.Host, cfg.Port, cfg.Database)
	}
	if cfg.TLSConfig == nil || len(cfg.TLSConfig.Certificates) != 1 || cfg.TLSConfig.VerifyPeerCertificate == nil || len(cfg.Fallbacks) != 0 {
		t.Fatalf("TLS = %+v, fallbacks %d: want the client certificate, a verified chain and no fallback", cfg.TLSConfig, len(cfg.Fallbacks))
	}
	if cfg.RuntimeParams["statement_timeout"] == "" {
		t.Fatal("no statement_timeout on the standby session")
	}
}

// The server is trusted when its chain reaches the database's CloudNativePG
// CA, or the roots of its issued public certificate through an intermediate
// it sends, whatever names it carries (an instance is dialed by Pod IP); a
// chain from any other CA is refused. A database with no CA Secret is not
// dialed at all.
func TestStandbySlotSweepVerifiesTheServerAgainstTheDatabasesCAs(t *testing.T) {
	cnpgCA := newTestCA(t, "dpg-ha-pub-ca", nil)
	publicRoot := newTestCA(t, "public root", nil)
	intermediate := newTestCA(t, "public intermediate", publicRoot)
	stranger := newTestCA(t, "stranger", nil)
	d := haPublisher("dpg-ha-pub")
	svc, _ := newService(d, replicationSecret(t, d.Name, cnpgCA), caSecret(d.Name+"-ca", cnpgCA), caSecret(d.Name+"-public-ca", publicRoot))
	cfg, err := svc.streamingReplicaTLS(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0]); err != nil || leaf.Subject.CommonName != cnpgStreamingReplicaUser {
		t.Fatalf("client certificate = %v, %v; want streaming_replica's", leaf, err)
	}
	cnpgLeaf, _ := cnpgCA.leaf(t, "dpg-ha-pub-rw", x509.ExtKeyUsageServerAuth)
	publicLeaf, _ := intermediate.leaf(t, "dpg-ha-pub.db.example.test", x509.ExtKeyUsageServerAuth)
	strangerLeaf, _ := stranger.leaf(t, "dpg-ha-pub-rw", x509.ExtKeyUsageServerAuth)
	for _, tc := range []struct {
		name  string
		chain [][]byte
		ok    bool
	}{
		{"CloudNativePG's CA", [][]byte{cnpgLeaf.Raw}, true},
		{"the public roots, through the intermediate sent", [][]byte{publicLeaf.Raw, intermediate.cert.Raw}, true},
		{"the public roots, without the intermediate", [][]byte{publicLeaf.Raw}, false},
		{"another CA", [][]byte{strangerLeaf.Raw}, false},
		{"no certificate", nil, false},
	} {
		if err := cfg.VerifyPeerCertificate(tc.chain, nil); (err == nil) != tc.ok {
			t.Errorf("%s: verify = %v, want ok %v", tc.name, err, tc.ok)
		}
	}

	bare, _ := newService(d, replicationSecret(t, d.Name, cnpgCA))
	if _, err := bare.streamingReplicaTLS(context.Background(), d); err == nil {
		t.Fatal("a database with no server CA got a TLS configuration")
	}
	w := &StandbySlotSweeper{Service: bare, sweepStandby: func(context.Context, *pgx.ConnConfig) ([]string, error) {
		t.Fatal("dialed a standby it cannot verify")
		return nil, nil
	}}
	if err := bare.Client.Create(context.Background(), instancePod("dpg-ha-pub-2", d.Name, cnpgReplicaInstanceRole, "10.0.0.2")); err != nil {
		t.Fatal(err)
	}
	if err := w.SweepOnce(context.Background()); err == nil {
		t.Fatal("the sweep hid a standby it could not verify")
	}
}

// A failing standby does not stop the others: the sweep reaches every one
// and reports each failure.
func TestStandbySlotSweepReportsEachFailingStandby(t *testing.T) {
	ca := newTestCA(t, "ca", nil)
	var objs []client.Object
	for i, name := range []string{"dpg-a", "dpg-b"} {
		objs = append(objs, haPublisher(name), replicationSecret(t, name, ca), caSecret(name+"-ca", ca),
			instancePod(name+"-2", name, cnpgReplicaInstanceRole, fmt.Sprintf("10.0.0.%d", i+2)))
	}
	svc, _ := newService(objs...)
	var reached []string
	w := &StandbySlotSweeper{Service: svc, sweepStandby: func(_ context.Context, cfg *pgx.ConnConfig) ([]string, error) {
		reached = append(reached, cfg.Host)
		if cfg.Host == "10.0.0.2" {
			return nil, context.DeadlineExceeded
		}
		return nil, nil
	}}
	err := w.SweepOnce(context.Background())
	slices.Sort(reached)
	if err == nil || !slices.Equal(reached, []string{"10.0.0.2", "10.0.0.3"}) {
		t.Fatalf("sweep = %v after reaching %v; want dpg-a's failure reported and both reached", err, reached)
	}
}
