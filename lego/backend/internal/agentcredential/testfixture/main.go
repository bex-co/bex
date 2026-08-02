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

// Command testfixture is a hermetic GitHub-compatible smart-HTTP origin and
// credential broker used only by lego/agent-image/credential-e2e-test.sh.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

const (
	repositoryRoot = "/srv/git"
	statusPath     = "/tmp/bex-agent-fixture-status"
)

type issuedCredential struct {
	expiresAt time.Time
	epoch     int
}

type fixture struct {
	mu                   sync.Mutex
	now                  time.Time
	tokens               map[string]issuedCredential
	mintCount            int
	logicalAdvances      int
	authenticatedCurrent int
}

func main() {
	log.SetFlags(0)
	if err := setupRepository(); err != nil {
		log.Fatalf("fixture repository: %v", err)
	}
	f := &fixture{now: time.Now().UTC(), tokens: map[string]issuedCredential{}}
	if err := f.writeStatus(); err != nil {
		log.Fatalf("fixture status: %v", err)
	}

	certificate, err := githubCertificate()
	if err != nil {
		log.Fatalf("fixture certificate: %v", err)
	}
	gitListener, err := tls.Listen("tcp", ":443", &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		log.Fatalf("fixture git listener: %v", err)
	}
	brokerListener, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatalf("fixture broker listener: %v", err)
	}

	gitServer := &http.Server{Handler: http.HandlerFunc(f.serveGit), ReadHeaderTimeout: 5 * time.Second}
	brokerMux := http.NewServeMux()
	brokerMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	brokerMux.HandleFunc("POST "+agentsession.GatewayPath, f.serveCredential)
	brokerServer := &http.Server{Handler: brokerMux, ReadHeaderTimeout: 5 * time.Second}
	go serve(gitServer, gitListener)
	go serve(brokerServer, brokerListener)
	log.Print("fixture ready")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM)
	for sig := range signals {
		if sig == syscall.SIGUSR1 {
			f.advance(61 * time.Minute)
			log.Print("fixture logical clock advanced beyond token TTL")
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = gitServer.Shutdown(ctx)
		_ = brokerServer.Shutdown(ctx)
		cancel()
		return
	}
}

func serve(server *http.Server, listener net.Listener) {
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Printf("fixture listener stopped: %v", err)
	}
}

func (f *fixture) serveCredential(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(agentsession.NamespaceHeader) != "tea-a-sandbox" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var request agentsession.MintRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&request); err != nil ||
		request.SessionID != "ags-verify" || request.Repository != "octo/repo" || request.Branch != "bex-agent/verify" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	f.mu.Lock()
	f.mintCount++
	issue := f.mintCount
	token := fmt.Sprintf("ghs_verify_%d", issue)
	expiresAt := f.now.Add(time.Hour)
	f.tokens[token] = issuedCredential{expiresAt: expiresAt, epoch: f.logicalAdvances}
	_ = f.writeStatusLocked()
	f.mu.Unlock()
	log.Printf("broker mint count=%d", issue)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(agentsession.MintResponse{
		Username: "x-access-token", Token: token, ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

func (f *fixture) serveGit(w http.ResponseWriter, r *http.Request) {
	username, token, ok := r.BasicAuth()
	if !ok || username != "x-access-token" || !f.authorize(token) {
		w.Header().Set("WWW-Authenticate", `Basic realm="bex-agent-fixture"`)
		http.Error(w, "credentials required", http.StatusUnauthorized)
		return
	}
	serveGitBackend(w, r)
}

func (f *fixture) authorize(token string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	credential, ok := f.tokens[token]
	if !ok || !credential.expiresAt.After(f.now) {
		return false
	}
	if credential.epoch == f.logicalAdvances && f.logicalAdvances > 0 {
		f.authenticatedCurrent++
		_ = f.writeStatusLocked()
	}
	return true
}

func (f *fixture) advance(delta time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(delta)
	f.logicalAdvances++
	_ = f.writeStatusLocked()
}

func (f *fixture) writeStatus() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writeStatusLocked()
}

func (f *fixture) writeStatusLocked() error {
	status := fmt.Sprintf("mint_count=%d\nlogical_advances=%d\nauthenticated_after_advance=%d\n", f.mintCount, f.logicalAdvances, f.authenticatedCurrent)
	return os.WriteFile(statusPath, []byte(status), 0o600)
}

func serveGitBackend(w http.ResponseWriter, r *http.Request) {
	command := exec.Command("git", "http-backend")
	command.Env = append(os.Environ(),
		"GIT_PROJECT_ROOT="+repositoryRoot,
		"GIT_HTTP_EXPORT_ALL=1",
		"PATH_INFO="+r.URL.Path,
		"QUERY_STRING="+r.URL.RawQuery,
		"REQUEST_METHOD="+r.Method,
		"CONTENT_TYPE="+r.Header.Get("Content-Type"),
		"CONTENT_LENGTH="+r.Header.Get("Content-Length"),
		"HTTP_GIT_PROTOCOL="+r.Header.Get("Git-Protocol"),
		"REMOTE_USER=x-access-token",
	)
	command.Stdin = r.Body
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		log.Printf("git-http-backend failed: %v: %s", err, strings.TrimSpace(stderr.String()))
		http.Error(w, "git backend unavailable", http.StatusBadGateway)
		return
	}
	writeCGIResponse(w, stdout.Bytes())
}

func writeCGIResponse(w http.ResponseWriter, response []byte) {
	separator, separatorLength := bytes.Index(response, []byte("\r\n\r\n")), 4
	if separator < 0 {
		separator, separatorLength = bytes.Index(response, []byte("\n\n")), 2
	}
	if separator < 0 {
		http.Error(w, "invalid git backend response", http.StatusBadGateway)
		return
	}
	status := http.StatusOK
	headers := strings.Split(strings.ReplaceAll(string(response[:separator]), "\r\n", "\n"), "\n")
	for _, header := range headers {
		key, value, ok := strings.Cut(header, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if strings.EqualFold(key, "Status") {
			fields := strings.Fields(value)
			if len(fields) > 0 {
				if code, err := strconv.Atoi(fields[0]); err == nil {
					status = code
				}
			}
			continue
		}
		w.Header().Add(key, value)
	}
	w.WriteHeader(status)
	_, _ = w.Write(response[separator+separatorLength:])
}

func setupRepository() error {
	repository := filepath.Join(repositoryRoot, "octo", "repo.git")
	if err := os.MkdirAll(filepath.Dir(repository), 0o755); err != nil {
		return err
	}
	if err := runGit("init", "--bare", repository); err != nil {
		return err
	}
	if err := runGit("--git-dir="+repository, "config", "http.receivepack", "true"); err != nil {
		return err
	}
	worktree, err := os.MkdirTemp("", "bex-agent-fixture-work-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(worktree)
	for _, args := range [][]string{
		{"-C", worktree, "init"},
		{"-C", worktree, "config", "user.name", "bex fixture"},
		{"-C", worktree, "config", "user.email", "fixture@bex.invalid"},
	} {
		if err := runGit(args...); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"-C", worktree, "add", "README.md"},
		{"-C", worktree, "commit", "-m", "initial"},
		{"-C", worktree, "branch", "-M", "main"},
		{"-C", worktree, "remote", "add", "origin", repository},
		{"-C", worktree, "push", "origin", "main"},
		{"--git-dir=" + repository, "symbolic-ref", "HEAD", "refs/heads/main"},
	} {
		if err := runGit(args...); err != nil {
			return err
		}
	}
	return nil
}

func runGit(args ...string) error {
	command := exec.Command("git", args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func githubCertificate() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "github.com"},
		DNSNames: []string{"github.com"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return tls.X509KeyPair(certPEM, keyPEM)
}
