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

package sshgateway

import (
	"context"
	"errors"
	"log"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A stream a bex API key opened is held to the key's grant for as long as it
// runs (w9/125, docs/ADR118-security-review-scoped-api-keys.md): bex-api
// refuses a key everything its grant does not cover on every request, and a
// shell, exec, file transfer or attach is a request that lasts. The gateway's
// Postgres role cannot read a grant, so it asks bex-api's HMAC verb
// (agentsession.InternalKeyGrantPath) at redemption and on every revalidation
// tick, uncached: a narrowing ends the stream within one
// BEX_SSH_REVALIDATE_INTERVAL. A ticket says it was minted by a key by naming
// the action its mint needed (core.KeyStreamAction); bex-api signs it, so
// nothing a client sends decides whether, or what, the gateway asks. A
// person's ticket names none and never reaches the verb, so a person's stream
// keeps exactly the behavior it had.

// KeyGrantAsker asks bex-api whether subject is a bound bex API key and
// whether its grant covers action. An error means the answer could not be had.
// *agentsession.KeyGrantClient satisfies it.
type KeyGrantAsker interface {
	Ask(ctx context.Context, subject, action string) (key, allowed bool, err error)
}

// KeyGrantGate is the gateway's hold on a key's grant. The zero value with an
// Asker is ready; a nil gate or Asker cannot ask, so every stream a key opened
// is refused (fail closed), as the workspace login-requirements gate refuses
// when it cannot ask.
type KeyGrantGate struct {
	Asker KeyGrantAsker
}

// RequireTicket refuses a ticket an API key minted under action (its signed
// KeyAction) once the key's grant no longer covers it: a coded 403 that wraps
// core.ErrForbidden, the same refusal bex-api answers the key's requests
// with. An answer that cannot be had is core.ErrAuthzUnavailable. A ticket
// that names no action (a person's, or bex-api's own) passes without asking.
func (g *KeyGrantGate) RequireTicket(ctx context.Context, subject, action string) error {
	if action == "" {
		return nil
	}
	_, err := g.require(ctx, subject, action)
	return err
}

// RequireSubject is RequireTicket for a principal with no ticket to say what
// it is: a native SSH key's registered owner, a person unless the key was
// enrolled by an API key before enrollment was refused to machine callers
// (ADR072 F3). It reports whether subject is a bound key, so the connection
// asks again on later checks only when it is.
func (g *KeyGrantGate) RequireSubject(ctx context.Context, subject, action string) (key bool, err error) {
	return g.require(ctx, subject, action)
}

func (g *KeyGrantGate) require(ctx context.Context, subject, action string) (bool, error) {
	if g == nil || g.Asker == nil {
		return false, unanswered(errors.New("no way to ask bex-api"))
	}
	key, allowed, err := g.Asker.Ask(ctx, subject, action)
	if err != nil {
		return false, unanswered(err)
	}
	if !allowed {
		return key, core.NewKeyPermissionDeniedError([]core.KeyAction{core.KeyAction(action)})
	}
	return key, nil
}

func unanswered(err error) error {
	log.Printf("ssh gateway: API key grant check unavailable: %v", err)
	return core.ErrAuthzUnavailable
}
