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

package core

import (
	"context"
	"errors"
	"log"
	"strings"
)

// An API key acts with its creator's current member role in the workspace it
// was minted in (w9/m481, docs/ADR024-members.md § An API key acts with its
// creator's role). Render: "Your key's access adjusts to match your new member
// role" (render.com/docs/api-keys). Two mechanisms hold it:
//
//   - the key's own role tuple mirrors its creator's role in that workspace,
//     seated at mint and re-seated by every role change through the durable
//     role outbox (internal/tenancy, internal/store, internal/members);
//   - the request-time ceiling below: a key is allowed a relation on an object
//     only when its creator is allowed the same relation on the same object at
//     the same moment, through the same cache or the same uncached check.
//
// The tuple keeps a key in its own workspace (it has none anywhere else, and
// the ceiling refuses a workspace other than the one it is bound to); the
// ceiling makes a lagging or failed tuple write unable to lift a key above its
// creator. Neither alone is enough: a tuple can lag, and the creator's own
// tuples reach every workspace they belong to.

// KeyCreatorResolver names who minted a bex API key. isKey=false means the
// client is not a bex API key at all (an operator-provisioned client, decided
// by its own tuples); isKey with an empty creator is a key minted before the
// created-by stamp existed. An error means the registry could not answer.
// twofactor.Gate implements it over its cached Hydra lookup.
type KeyCreatorResolver interface {
	APIKeyCreator(ctx context.Context, clientID string) (creator string, isKey bool, err error)
}

// maxKeyChain bounds the creator walk. A key minted by another key (possible
// only before codex round-7 F3 made minting human-only) is capped by that
// key's own decision in turn; nothing legitimate is deeper than that.
const maxKeyChain = 3

// unattributedKeyRelations is everything a key with no recorded creator may
// still do: the relations a Developer holds, its binding before w9/m481. Such a
// key never gains more (invariant I3), whatever tuple it holds.
var unattributedKeyRelations = map[string]bool{
	RelCanView:          true,
	RelCanViewLogs:      true,
	RelCanOperate:       true,
	RelCanCreate:        true,
	RelCanViewSensitive: true,
	RelCanManageKeys:    true,
	RelCanManageSSHKeys: true,
}

// checkFunc is one OpenFGA decision for an explicit subject: the cached
// Checker.Check or the uncached FreshChecker.CheckFresh.
type checkFunc func(ctx context.Context, subject, relation, object string) (bool, error)

// keyCeiling is the request-time half of the rule above, called by checkAuthz
// and checkAuthzFresh after the caller's own positive decision, with the same
// check function that decision used. Anything that is not an API key, or a
// deployment with no KeyCreators, passes untouched.
func (b *Base) keyCeiling(ctx context.Context, id Identity, relation, object string, check checkFunc) error {
	if b.KeyCreators == nil || !id.IsAPIKey() {
		return nil
	}
	// I2 in depth: a key's tuples exist only in the workspace it is bound to,
	// and a workspace decision is refused anywhere else even if a stray tuple
	// (a hand-written grant) says otherwise. The resolution is the cached one
	// every request already makes.
	if workspaceID, isWorkspace := strings.CutPrefix(object, WorkspaceObjectPrefix); isWorkspace && b.Workspace != nil {
		if home, bound := b.Workspace.Tenant(ctx, id); !bound || home != workspaceID {
			return ErrForbidden
		}
	}
	clientID := id.KeyClientID()
	for range maxKeyChain {
		creator, isKey, err := b.KeyCreators.APIKeyCreator(ctx, clientID)
		switch {
		case err != nil:
			log.Printf("core: API key creator lookup unavailable: %v", err)
			return ErrAuthzUnavailable
		case !isKey:
			// clientID is a person, or an operator-provisioned client: the
			// decision already made for it stands.
			return nil
		case creator == "":
			if !unattributedKeyRelations[relation] {
				return ErrForbidden
			}
			return nil
		}
		allowed, err := check(ctx, "user:"+creator, relation, object)
		if err != nil {
			log.Printf("core: authorization unavailable: %v", err)
			return ErrAuthzUnavailable
		}
		if !allowed {
			return ErrForbidden
		}
		clientID = creator
	}
	return ErrForbidden
}

// KeyCreator names who minted the caller when it is a bex API key. isKey is
// false for every other caller (a person, a delegated OAuth token, an
// operator-provisioned client, a deployment with no key registry); creator is
// "" for a key minted before the created-by stamp existed. An error means the
// key registry could not answer.
func (b *Base) KeyCreator(ctx context.Context) (creator string, isKey bool, err error) {
	id, ok := IdentityFrom(ctx)
	if !ok || !id.IsAPIKey() || b.KeyCreators == nil {
		return "", false, nil
	}
	creator, isKey, err = b.KeyCreators.APIKeyCreator(ctx, id.KeyClientID())
	if err != nil {
		log.Printf("core: API key creator lookup unavailable: %v", err)
		return "", false, ErrAuthzUnavailable
	}
	return creator, isKey, nil
}

// AttributedAPIKey reports whether the caller is a bex API key with a recorded
// creator: a first-party credential a person minted through the mint-class
// gate, revoked with that person's membership. An error means the key registry
// could not answer. It is false for every other caller, including any other
// client_credentials client.
func (b *Base) AttributedAPIKey(ctx context.Context) (bool, error) {
	creator, isKey, err := b.KeyCreator(ctx)
	return isKey && creator != "", err
}

// AuthorizeExportClass gates a verb that exports the workspace's activity under
// a durable secret (a webhook endpoint's create and repoint), that changes a
// member's role (w9/116), or that issues a Postgres login and returns its
// password (w9/117), on the credential class: AuthorizeMintClass's callers
// plus a bex API key with a recorded creator, which Render's keys are (its
// permissions include "Create, update, and delete webhooks", "Update member
// roles and remove members" and "Create and delete database credentials"). A
// delegated third-party OAuth token and any other machine client stay refused
// (w4/079, round 24 H-1). Like AuthorizeMintClass it decides no role: the
// verb's relation check does, and for a key that check is already capped by
// its creator.
func (b *Base) AuthorizeExportClass(ctx context.Context) error {
	if id, ok := IdentityFrom(ctx); ok && id.IsAPIKey() {
		if err := revalidateBearer(ctx); err != nil {
			return err
		}
		attributed, err := b.AttributedAPIKey(ctx)
		if err != nil {
			return err
		}
		if !attributed {
			return ErrForbidden
		}
		return nil
	}
	return b.AuthorizeMintClass(ctx)
}

// HoldsFresh reports, audit-silent and uncached, whether the caller holds
// relation on object: the form a field-level projection uses (who sees a
// webhook's secret, a workspace's billing figures, every sandbox) instead of
// asking the checker directly, so an API key's answer is capped by its creator
// there too. A refusal, the OAuth capability's included, is (false, nil); a
// check that could not answer is ErrAuthzUnavailable. The caller decides what
// a nil checker means before asking.
func (b *Base) HoldsFresh(ctx context.Context, relation, object string) (bool, error) {
	err := b.checkAuthzFresh(ctx, relation, object)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrAuthzUnavailable):
		return false, err
	default:
		return false, nil
	}
}
