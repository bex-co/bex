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

package s3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/render-oss/cli/pkg/config"
)

const (
	// keyLife is how long the ephemeral key bex mints for the signed-in person
	// lives (ADR112 D3; the API caps it at twelve hours).
	keyLife = 12 * time.Hour
	// renewWindow is when a cached key is replaced: under an hour left is
	// too little for a transfer, and a command that outlives its key fails
	// with a 403 that is not retried.
	renewWindow = time.Hour
	// defaultRegion is the signing region of a request. bex compares it with
	// nothing (ADR112 D1); `--region` overrides it, and `mb` takes the
	// bucket's location from it.
	defaultRegion = "oregon"
	// productionEndpoint is the S3 endpoint of the production control plane.
	// It is a host of the platform's hosting domain, which is not the API's:
	// api.bex.co and objects.onbex.co (ADR112 D3).
	productionEndpoint = "https://objects.onbex.co"
	// endpointEnv names a control plane's S3 endpoint when it is not the
	// production one.
	endpointEnv = "BEX_S3_ENDPOINT"
	// accessKeyEnv and secretKeyEnv hand bex a persistent key (`bex s3 keys
	// create`) in place of the twelve-hour key it makes for a signed-in person:
	// the way CI, which has an API key and no login, uses `bex s3` (ADR112 D3).
	accessKeyEnv = "BEX_S3_ACCESS_KEY_ID"
	secretKeyEnv = "BEX_S3_SECRET_ACCESS_KEY"
)

// session is everything a command needs to reach the S3 endpoint as the
// signed-in person.
type session struct {
	// Workspace is empty when the key came from the environment: such a key
	// needs no sign-in and no workspace.
	Workspace   string
	Endpoint    string
	Region      string
	AccessKeyID string
	Secret      string
	// ExpiresAt is zero for a persistent key.
	ExpiresAt time.Time
	// CachePath is the file the ephemeral key is kept in, empty for a key from
	// the environment.
	CachePath string
}

// cachedKey is the ephemeral key kept between invocations. It lives in the
// state directory with mode 0600.
type cachedKey struct {
	Host        string    `json:"host"`
	Workspace   string    `json:"workspace"`
	AccessKeyID string    `json:"accessKeyId"`
	Secret      string    `json:"secret"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// owner is the active workspace, which every route addresses.
func (d deps) owner() (string, error) {
	id, err := d.workspace()
	if err != nil {
		if errors.Is(err, config.ErrNoWorkspace) {
			return "", errors.New("no active workspace: run `bex workspace set`, or set BEX_WORKSPACE")
		}
		return "", err
	}
	return id, nil
}

// endpoint is the S3 endpoint of the control plane in use.
func (d deps) endpoint() (string, error) {
	if value, ok := d.lookupEnv(endpointEnv); ok && value != "" {
		return normalizeEndpoint(value)
	}
	if sameHostname(d.host(), productionHost) {
		return productionEndpoint, nil
	}
	return "", fmt.Errorf("the S3 endpoint of %s is not known: set %s to https://objects.<its platform hosting domain>", d.host(), endpointEnv)
}

// productionHost is the API base the production endpoint belongs to.
const productionHost = "https://api.bex.co/v1/"

func sameHostname(a, b string) bool {
	ua, errA := url.Parse(strings.TrimSpace(a))
	ub, errB := url.Parse(strings.TrimSpace(b))
	return errA == nil && errB == nil && ua.Hostname() != "" && strings.EqualFold(ua.Hostname(), ub.Hostname())
}

// normalizeEndpoint accepts an origin only: https, or http to a loopback
// address for development. A key is never sent in clear over a network.
func normalizeEndpoint(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return "", fmt.Errorf("%s must be an origin such as https://objects.example.com, not %q", endpointEnv, raw)
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && isLoopback(u.Hostname()):
	default:
		return "", fmt.Errorf("%s must be https (http is accepted for localhost only), not %q", endpointEnv, raw)
	}
	return u.Scheme + "://" + u.Host, nil
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// stateDir is where bex keeps the S3 key.
func (d deps) stateDir() (string, error) {
	state, err := d.state()
	if err != nil {
		return "", fmt.Errorf("resolve the state directory: %w", err)
	}
	dir := filepath.Join(state, "s3")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return dir, nil
}

// keyFile names the cache of one workspace on one control plane.
func keyFile(dir, host, workspace string) string {
	sum := sha256.Sum256([]byte(host + "\x00" + workspace))
	return filepath.Join(dir, "key-"+hex.EncodeToString(sum[:6])+".json")
}

// loadKey is the cached key when it is for this control plane and workspace
// and has more than renewWindow left. A missing, damaged or stale file reads
// as no key: the next step mints one.
func loadKey(path, host, workspace string, now time.Time) (cachedKey, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return cachedKey{}, false
	}
	var key cachedKey
	if json.Unmarshal(raw, &key) != nil || key.AccessKeyID == "" || key.Secret == "" {
		return cachedKey{}, false
	}
	if key.Host != host || key.Workspace != workspace || key.ExpiresAt.Sub(now) <= renewWindow {
		return cachedKey{}, false
	}
	return key, true
}

// saveKey writes the key owner-only, through a rename so a reader never sees
// half of it.
func saveKey(path string, key cachedKey) error {
	raw, err := json.Marshal(key)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw)
}

// writeFileAtomic replaces path with data, mode 0600.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}

// lockFile takes an exclusive advisory lock on path until the returned release runs.
func lockFile(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() { _ = f.Close() }, nil // closing the file releases the lock
}

// envKey is the persistent key of the environment, if there is one. Setting
// one of the two names without the other is a mistake worth saying so.
func (d deps) envKey() (id, secret string, ok bool, err error) {
	id, _ = d.lookupEnv(accessKeyEnv)
	secret, _ = d.lookupEnv(secretKeyEnv)
	switch {
	case id == "" && secret == "":
		return "", "", false, nil
	case id == "" || secret == "":
		return "", "", false, fmt.Errorf("set both %s and %s, or neither", accessKeyEnv, secretKeyEnv)
	}
	return id, secret, true, nil
}

// session resolves the endpoint and a key: the persistent key of the
// environment when it names one, else the cached ephemeral key of the active
// workspace while it has more than an hour left, else a fresh one minted for
// the signed-in person. region is the signing region, "" for the default.
func (d deps) session(ctx context.Context, region string) (session, error) {
	if region == "" {
		region = defaultRegion
	}
	endpoint, err := d.endpoint()
	if err != nil {
		return session{}, err
	}
	if id, secret, ok, err := d.envKey(); err != nil {
		return session{}, err
	} else if ok {
		return session{Endpoint: endpoint, Region: region, AccessKeyID: id, Secret: secret}, nil
	}
	workspace, err := d.owner()
	if err != nil {
		return session{}, err
	}
	dir, err := d.stateDir()
	if err != nil {
		return session{}, err
	}
	host := d.host()
	path := keyFile(dir, host, workspace)
	// One mint at a time: a script that starts many `bex s3` commands at once on a machine
	// with no key would otherwise make one each and hit the limit of 20 live ephemeral keys.
	unlock, err := lockFile(path + ".lock")
	if err != nil {
		return session{}, fmt.Errorf("lock %s: %w", path, err)
	}
	defer unlock()
	key, ok := loadKey(path, host, workspace, d.now())
	if !ok {
		if key, err = d.mintKey(ctx, host, workspace); err != nil {
			return session{}, err
		}
		if err := saveKey(path, key); err != nil {
			return session{}, fmt.Errorf("keep the S3 access key in %s: %w", dir, err)
		}
	}
	return session{
		Workspace: workspace, Endpoint: endpoint, Region: region,
		AccessKeyID: key.AccessKeyID, Secret: key.Secret, ExpiresAt: key.ExpiresAt, CachePath: path,
	}, nil
}

// mintKey asks bex-api for an ephemeral read-write key for the signed-in
// person in workspace.
func (d deps) mintKey(ctx context.Context, host, workspace string) (cachedKey, error) {
	a, err := d.api()
	if err != nil {
		return cachedKey{}, err
	}
	created, _, err := a.createKey(ctx, workspace, createKeyRequest{
		Name: "bex s3" + hostSuffix(d.hostname()), Permission: "readwrite", ExpiresInSeconds: int64(keyLife / time.Second),
	})
	if err != nil {
		return cachedKey{}, mintError(workspace, err)
	}
	if created.ID == "" || created.Secret == "" || created.ExpiresAt == nil {
		return cachedKey{}, errors.New("bex-api answered without an access key, its secret or its expiry")
	}
	expires, err := time.Parse(time.RFC3339, *created.ExpiresAt)
	if err != nil {
		return cachedKey{}, fmt.Errorf("the new access key's expiry %q is not a time: %w", *created.ExpiresAt, err)
	}
	return cachedKey{Host: host, Workspace: workspace, AccessKeyID: created.ID, Secret: created.Secret, ExpiresAt: expires}, nil
}

// hostSuffix names the machine in a key's name, so `bex s3 keys list` tells
// the keys of two laptops apart. Control characters would be refused.
func hostSuffix(hostname string) string {
	var b strings.Builder
	for _, r := range hostname {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
		if b.Len() >= 60 {
			break
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return " on " + b.String()
}

// mintError says what a refused mint needs, in the person's terms.
func mintError(workspace string, err error) error {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("create an S3 access key in %s: %w", workspace, err)
	}
	switch apiErr.Status {
	case 401, 403:
		return fmt.Errorf("create an S3 access key in %s: %w\n"+
			"A key is made for a person, with write access to the workspace: sign in with `bex login` "+
			"(an API key, BEX_API_KEY, cannot make one; make a persistent key with `bex s3 keys create` "+
			"from a signed-in session and set %s and %s to it)", workspace, err, accessKeyEnv, secretKeyEnv)
	case 404:
		return fmt.Errorf("create an S3 access key in %s: %w\nobject storage is not available on this control plane", workspace, err)
	}
	return fmt.Errorf("create an S3 access key in %s: %w", workspace, err)
}
