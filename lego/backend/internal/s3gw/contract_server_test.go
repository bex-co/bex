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

package s3gw

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// The server half of scripts/s3-contract.sh (ADR112 D11, layer 3). The script runs the
// pinned aws-cli v2, rclone and boto3, and a bex built from lego/cli, against this gateway over
// TLS (the clients' framing differs over plain HTTP, where they sign the payload instead of sending a trailer), on
// real Postgres and a fake Wasabi. This test only serves: it writes where it listens and
// the key to use, logs what each client sent and what bex answered, and when the script is
// done it checks that no client was answered with a 5xx and writes the report of the
// defaults the clients used and of what bex answered 501. It is skipped without BEX_S3_CONTRACT_DIR.
// With BEX_S3_CONTRACT_EDGE_PORT the clients' endpoint is that port, where the script runs the edge's
// Traefik in front of this gateway, and the test also writes the certificate's key for it.

const (
	contractRegion = "oregon"
	// contractDeadline bounds a run that the script never ends.
	contractDeadline = 25 * time.Minute
)

// contractRecord is one request the gateway saw.
type contractRecord struct {
	Phase    string   `json:"phase"`
	Client   string   `json:"client"`
	Method   string   `json:"method"`
	Host     string   `json:"host"`
	Path     string   `json:"path"`
	Query    []string `json:"query"`
	Payload  string   `json:"payload"`
	Encoding string   `json:"contentEncoding"`
	Trailer  string   `json:"trailer"`
	Checksum []string `json:"checksumHeaders"`
	Chunked  bool     `json:"chunked"`
	Expect   bool     `json:"expect100"`
	Status   int      `json:"status"`
}

type recordingWriter struct {
	http.ResponseWriter
	status int
}

func (w *recordingWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *recordingWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *recordingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// clientOf names the client a User-Agent belongs to.
func clientOf(userAgent string) string {
	switch ua := strings.ToLower(userAgent); {
	case strings.Contains(ua, "aws-cli/"):
		return "aws-cli"
	case strings.Contains(ua, "boto3/"), strings.Contains(ua, "botocore/"):
		return "boto3"
	case strings.HasPrefix(ua, "rclone/"):
		return "rclone"
	case strings.Contains(ua, "app/bex-cli"):
		return "bex"
	}
	return "other"
}

// TestClientOfNamesTheFourClients: bex's own client is told apart by the application id its
// requests carry, so its 5xx answers fail the run and its defaults are in the report.
func TestClientOfNamesTheFourClients(t *testing.T) {
	for ua, want := range map[string]string{
		"aws-cli/2.39.5 md/awscrt#0.28 ua/2.1 os/linux#6.8 md/arch#arm64 lang/python#3.13 md/pyimpl#CPython cfg/retry-mode#standard md/installer#exe md/prompt#off md/command#s3.cp": "aws-cli",
		"Boto3/1.43.125 md/Botocore#1.43.125 ua/2.1 os/linux#6.8 lang/python#3.12 api/s3#1.43.125 Boto3/1.43.125":                                                                    "boto3",
		"rclone/v1.75.2": "rclone",
		"aws-sdk-go-v2/1.48.0 ua/2.1 os/linux lang/go#1.27.3 md/GOOS#linux md/GOARCH#arm64 api/s3#1.116.0 app/bex-cli m/E,Z,e": "bex",
		"aws-sdk-go-v2/1.48.0 ua/2.1 os/linux lang/go#1.27.3 api/s3#1.116.0":                                                   "other",
		"curl/8.5.0": "other",
	} {
		if got := clientOf(ua); got != want {
			t.Errorf("clientOf(%q) = %q, want %q", ua, got, want)
		}
	}
}

// payloadMode is x-amz-content-sha256 without a digest, which names the mode.
func payloadMode(v string) string {
	switch {
	case v == "":
		return "none"
	case strings.HasPrefix(v, "STREAMING-"), v == "UNSIGNED-PAYLOAD":
		return v
	}
	return "sha256-of-body"
}

// contractLog is the request log of a run: JSON lines in the run's directory.
type contractLog struct {
	mu   sync.Mutex
	dir  string
	file *os.File
	rows []contractRecord
}

func (l *contractLog) currentPhase() string {
	raw, err := os.ReadFile(filepath.Join(l.dir, "phase"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(raw))
}

func (l *contractLog) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := contractRecord{
			Phase: l.currentPhase(), Client: clientOf(r.UserAgent()), Method: r.Method, Host: r.Host, Path: r.URL.EscapedPath(),
			Payload: payloadMode(r.Header.Get("X-Amz-Content-Sha256")), Encoding: r.Header.Get("Content-Encoding"),
			Trailer: r.Header.Get("X-Amz-Trailer"), Chunked: len(r.TransferEncoding) > 0,
			Expect: strings.EqualFold(r.Header.Get("Expect"), "100-continue"),
		}
		for name := range r.URL.Query() {
			if !strings.HasPrefix(name, "X-Amz-") {
				rec.Query = append(rec.Query, name)
			}
		}
		for name := range r.Header {
			if strings.HasPrefix(strings.ToLower(name), "x-amz-checksum-") || strings.EqualFold(name, "X-Amz-Sdk-Checksum-Algorithm") {
				rec.Checksum = append(rec.Checksum, strings.ToLower(name))
			}
		}
		sort.Strings(rec.Query)
		sort.Strings(rec.Checksum)
		rw := &recordingWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		rec.Status = rw.status
		if rec.Status == 0 {
			rec.Status = http.StatusOK
		}
		l.append(rec)
	})
}

func (l *contractLog) append(rec contractRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rows = append(l.rows, rec)
	line, _ := json.Marshal(rec)
	_, _ = l.file.Write(append(line, '\n'))
}

// addressing is the style a request used: the endpoint's own host is path style.
func (rec contractRecord) addressing(endpointHost string) string {
	if rec.Host == endpointHost {
		return "path"
	}
	return "virtual-hosted (" + rec.Host + ")"
}

// report is the defaults each client sent, one line per distinct shape per phase and
// client, with the number of requests and the answers they got.
func (l *contractLog) report(endpointHost string) (string, []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	type shape struct{ phase, client, addressing, payload, encoding, trailer, checksum, chunked, expect string }
	counts := map[shape]map[int]int{}
	refused := map[string]int{}
	var problems []string
	for _, rec := range l.rows {
		if rec.Client == "other" {
			continue
		}
		s := shape{rec.Phase, rec.Client, rec.addressing(endpointHost), rec.Payload, rec.Encoding, rec.Trailer, strings.Join(rec.Checksum, ","), fmt.Sprint(rec.Chunked), fmt.Sprint(rec.Expect)}
		if counts[s] == nil {
			counts[s] = map[int]int{}
		}
		counts[s][rec.Status]++
		// A 501 is a client sending something bex refuses: the client's own exit status says
		// whether it minded (rclone asks for the bucket's versioning and goes on without it),
		// and the report lists each one for a person to read at a pin change. A 5xx is bex
		// failing, and fails the run; 503 is SlowDown, which clients retry.
		if rec.Status == http.StatusNotImplemented && rec.Phase != "probe" && rec.Phase != "refused" {
			refused[fmt.Sprintf("%s %s %s?%s", rec.Client, rec.Method, rec.Path, strings.Join(rec.Query, "&"))]++
		}
		if rec.Status >= 500 && rec.Status != http.StatusNotImplemented && rec.Status != http.StatusServiceUnavailable {
			problems = append(problems, fmt.Sprintf("%s (%s) %s %s was answered %d", rec.Client, rec.Phase, rec.Method, rec.Path, rec.Status))
		}
	}
	var lines []string
	for s, answers := range counts {
		var statuses []string
		total := 0
		for code, n := range answers {
			statuses = append(statuses, fmt.Sprintf("%dx%d", n, code))
			total += n
		}
		sort.Strings(statuses)
		lines = append(lines, fmt.Sprintf("%-8s %-10s addressing=%s payload=%s content-encoding=%q trailer=%q checksum-headers=%q chunked=%s expect-100=%s: %d requests (%s)",
			s.client, s.phase, s.addressing, s.payload, s.encoding, s.trailer, s.checksum, s.chunked, s.expect, total, strings.Join(statuses, " ")))
	}
	sort.Strings(lines)
	var notImplemented []string
	for request, n := range refused {
		notImplemented = append(notImplemented, fmt.Sprintf("answered 501 to %d request(s) of %s", n, request))
	}
	sort.Strings(notImplemented)
	return strings.Join(append(lines, notImplemented...), "\n") + "\n", problems
}

// selfSigned is a certificate for objects.localhost and the subdomains a virtual-hosted
// request would use (<bucket>.objects.localhost), so a client that picks that style still
// verifies the name, and its PEM for the clients to trust.
func selfSigned(t *testing.T) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "bex s3 contract"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
		DNSNames:    []string{"localhost", "*.localhost", "objects.localhost", "*.objects.localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestS3ContractServer(t *testing.T) {
	dir := os.Getenv("BEX_S3_CONTRACT_DIR")
	if dir == "" {
		t.Skip("BEX_S3_CONTRACT_DIR is not set: this is the server half of scripts/s3-contract.sh")
	}
	p := newWideRig(t)
	p.gw.cfg.Now = time.Now
	p.keys.Now = time.Now
	logFile, err := os.Create(filepath.Join(dir, "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logFile.Close() })
	reqLog := &contractLog{dir: dir, file: logFile}

	cert, caPEM := selfSigned(t)
	srv := httptest.NewUnstartedServer(reqLog.wrap(p.gw))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	// objects.localhost, with a bucket label in front of it, is the shape of production's
	// <bucket>.objects.onbex.co: where the system resolves it, a client that picks
	// virtual-hosted style reaches this server and the report says so.
	host := "localhost"
	if addrs, err := net.LookupHost("probe.objects.localhost"); err == nil && len(addrs) > 0 {
		host = "objects.localhost"
	}
	// scripts/s3-contract.sh puts Traefik, with the plugin of the S3 route, in front of the
	// gateway as the edge does in production (w21/113): the clients then talk to its port.
	// The gateway's own port stays the endpoint when the variable is unset.
	clientPort := port
	var edgeKey []byte
	if edge := os.Getenv("BEX_S3_CONTRACT_EDGE_PORT"); edge != "" {
		if clientPort, err = strconv.Atoi(edge); err != nil {
			t.Fatalf("BEX_S3_CONTRACT_EDGE_PORT = %q: %v", edge, err)
		}
		if edgeKey, err = x509.MarshalPKCS8PrivateKey(cert.PrivateKey); err != nil {
			t.Fatal(err)
		}
	}
	endpointHost := fmt.Sprintf("%s:%d", host, clientPort)

	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	if err := os.WriteFile(filepath.Join(dir, "ca.pem"), caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if edgeKey != nil {
		// The edge serves the same certificate, which names the same hosts, and forwards to the gateway.
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: edgeKey})
		if err := os.WriteFile(filepath.Join(dir, "edge.key"), keyPEM, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	env := fmt.Sprintf("S3_ENDPOINT=https://%s\nS3_GATEWAY_PORT=%d\nS3_ACCESS_KEY_ID=%s\nS3_SECRET_ACCESS_KEY=%s\nS3_REGION=%s\n", endpointHost, port, k.id, k.secret, contractRegion)
	// The environment file appears last: the script waits for it.
	if err := os.WriteFile(filepath.Join(dir, "server.env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(contractDeadline)
	for {
		select {
		case <-deadline:
			t.Fatalf("scripts/s3-contract.sh did not finish within %s", contractDeadline)
		case <-time.After(200 * time.Millisecond):
		}
		if _, err := os.Stat(filepath.Join(dir, "stop")); err == nil {
			break
		}
	}
	report, problems := reqLog.report(endpointHost)
	if err := os.WriteFile(filepath.Join(dir, "report.txt"), []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

// TestContractReportFailsOnA5xxAndListsA501: the verdict of scripts/s3-contract.sh. A client
// answered 5xx fails the run, whichever phase it was in; a 501 is listed for a person to read
// (rclone asks for versioning and goes on without it) and fails nothing; a refusal the script
// asked for, and the probe of what a client's defaults do, are not findings.
func TestContractReportFailsOnA5xxAndListsA501(t *testing.T) {
	l := &contractLog{}
	add := func(phase, client string, status int, query ...string) {
		l.rows = append(l.rows, contractRecord{Phase: phase, Client: client, Method: "GET", Host: "objects.localhost:1", Path: "/b", Query: query, Status: status})
	}
	add("rclone", "rclone", 200)
	add("rclone", "rclone", 501, "versioning")
	add("refused", "aws-cli", 501, "versioning")
	add("probe", "aws-cli", 501)
	add("other", "other", 500) // not one of the four clients
	report, problems := l.report("objects.localhost:1")
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none for a 501 the client tolerates", problems)
	}
	if !strings.Contains(report, "answered 501 to 1 request(s) of rclone GET /b?versioning") || strings.Contains(report, "aws-cli GET") {
		t.Errorf("the report lists the wrong 501s:\n%s", report)
	}

	add("aws", "aws-cli", 500)
	add("boto3", "boto3", 502)
	add("boto3", "boto3", 503) // SlowDown: clients retry it
	add("bex", "bex", 500)
	if _, problems = l.report("objects.localhost:1"); len(problems) != 3 {
		t.Errorf("problems = %v, want the 500 and the 502 of the stock clients and the 500 of bex", problems)
	}
	add("aws", "aws-cli", 200)
	l.rows[len(l.rows)-1].Host = "photos.objects.localhost:1"
	if report, _ = l.report("objects.localhost:1"); !strings.Contains(report, "addressing=virtual-hosted (photos.objects.localhost:1)") {
		t.Errorf("a virtual-hosted request is not named in the report:\n%s", report)
	}
}
