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

package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/render-oss/cli/pkg/client/oauth"
	"github.com/render-oss/cli/pkg/config"
)

// A `bex login` access token lives seven days (the CLI's OAuth client,
// scripts/auth-bootstrap-client.sh), and upstream refreshes it on every
// command in its last day (client.NewDefaultClient). Each refresh revokes the
// access token before it at once, so one `bex` command in a second terminal
// used to sign out a running `bex agent` (w3/m404). login keeps the bearer of
// one run valid instead, sharing the login file with every other bex process.

const (
	// renewBefore is how long before its expiry the bearer is refreshed.
	renewBefore = 2 * time.Minute
	// refreshTimeout bounds one refresh, as upstream bounds its own.
	refreshTimeout = 5 * time.Second
	// refreshPause spaces out refreshes that failed while the bearer still
	// works, so a token endpoint outage costs one attempt per pause rather
	// than one per request.
	refreshPause = 30 * time.Second
)

// login is the credential every request of one run carries.
//
// Before each request it re-reads the login file and adopts a newer login
// another process wrote there, which costs no refresh and so revokes nobody's
// token. It refreshes only when the bearer is about to expire or the API
// refused it, from the refresh token on file (the newest one), and writes the
// result back only while the file still holds the login it refreshed from. A
// failed refresh writes nothing: upstream's own failure path clears the
// refresh token on file, which can erase a newer login another process wrote.
type login struct {
	mu  sync.Mutex
	cur config.APIConfig
	now func() time.Time
	// renewBefore is the package's renewBefore; tests shorten it.
	renewBefore time.Duration
	// spent is the refresh token this run last traded, so a file that still
	// holds it (a write that failed) is never refreshed from again.
	spent string
	// pausedUntil delays the next refresh after one failed.
	pausedUntil time.Time
}

func newLogin(cur config.APIConfig) *login {
	return &login{cur: cur, now: time.Now, renewBefore: renewBefore}
}

// errLoginEnded is a login that cannot be renewed here: it was signed out,
// revoked, or never had a refresh token. Only `bex login` mends it.
var errLoginEnded = errors.New("the bex login has ended; run `bex login`, then run this command again")

// bearer is the access token to send now; "" sends none.
func (l *login) bearer(ctx context.Context) (string, error) {
	if l == nil {
		return "", nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !renewable(l.cur) {
		return l.cur.Key, nil // an API key, or a login without an expiry
	}
	disk, ok := l.read()
	l.adopt(disk, ok)
	now := l.now()
	if !l.expiring(l.cur, now) || now.Before(l.pausedUntil) {
		return l.cur.Key, nil
	}
	err := l.renew(ctx, disk, ok)
	if err == nil {
		return l.cur.Key, nil
	}
	if now.Unix() < l.cur.ExpiresAt {
		// Still valid: keep sending it, and try again after a pause.
		l.pausedUntil = now.Add(refreshPause)
		return l.cur.Key, nil
	}
	return "", err
}

// rejected renews the login after the API answered 401 to sent. retry reports
// that the next request carries a different bearer; neither retry nor err is
// set for an API key, which has nothing to renew, or without a login.
func (l *login) rejected(ctx context.Context, sent string) (retry bool, err error) {
	if l == nil {
		return false, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cur.Key != sent {
		return true, nil // another request of this run renewed it meanwhile
	}
	if !renewable(l.cur) {
		return false, nil
	}
	disk, ok := l.read()
	if l.adopt(disk, ok) {
		return true, nil
	}
	if err := l.renew(ctx, disk, ok); err != nil {
		return false, err
	}
	return true, nil
}

// renewable reports whether c is a login that expires, rather than an API key.
func renewable(c config.APIConfig) bool {
	return c.ExpiresAt > 0
}

// expiring reports whether c expires within renewBefore of now.
func (l *login) expiring(c config.APIConfig, now time.Time) bool {
	return now.Add(l.renewBefore).Unix() >= c.ExpiresAt
}

// read is the login on file now. ok is false when there is none to use: the
// file is unreadable, or it holds no login.
func (l *login) read() (config.APIConfig, bool) {
	disk, err := config.DefaultAPIConfig()
	return disk, err == nil && disk.Key != ""
}

// adopt switches to the login on file when another process wrote a newer one
// for the same API that is not about to expire, and reports whether it did.
func (l *login) adopt(disk config.APIConfig, ok bool) bool {
	if !ok || disk.Key == l.cur.Key || disk.Host != l.cur.Host || !renewable(disk) || l.expiring(disk, l.now()) {
		return false
	}
	l.cur = disk
	return true
}

// renew refreshes the login. It trades the refresh token on file, which is
// the newest one, unless this run already traded it (its write back failed);
// a refresh token used again after the server's grace period revokes the
// whole login, every process's token with it.
func (l *login) renew(ctx context.Context, disk config.APIConfig, ok bool) error {
	from := l.cur
	if ok && disk.RefreshToken != "" && disk.RefreshToken != l.spent && disk.Host == l.cur.Host {
		from = disk
	}
	if from.RefreshToken == "" {
		return errLoginEnded
	}
	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	token, err := oauth.NewClient(from.Host).RefreshToken(ctx, from.RefreshToken)
	if err != nil {
		if transient(err) {
			return fmt.Errorf("renew the bex login: %w", err)
		}
		return fmt.Errorf("renew the bex login: %w; run `bex login`, then run this command again", err)
	}
	next := from
	next.Key, next.RefreshToken = token.AccessToken, token.RefreshToken
	next.ExpiresAt = l.now().Add(time.Duration(token.ExpiresIn) * time.Second).Unix()
	l.spent, l.cur, l.pausedUntil = from.RefreshToken, next, time.Time{}
	// A failed write leaves this run's token working; the next refresh then
	// starts from this run's own refresh token (spent guards it).
	_ = save(from, next)
	return nil
}

// save writes next unless the file no longer holds prev: another process
// signed in, out, or renewed meanwhile, and its login is the one to keep.
func save(prev, next config.APIConfig) error {
	disk, err := config.OAuthConfig()
	if err != nil {
		return err
	}
	if disk.Key != prev.Key || disk.RefreshToken != prev.RefreshToken {
		return nil
	}
	return config.SetAPIConfig(next)
}
