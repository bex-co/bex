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

package store

import (
	"context"
)

// gitwebhook.go is the durable replay ledger for the signed Git push webhook
// (codex round-8 #9). The webhook's HMAC verifies the body's bytes; nothing
// about that proves the delivery is FRESH. These claims close the replay path:
// the exact signed bytes are claimed once before any mutation, so a captured
// (body, signature) pair — however it is resent — can never re-enter
// redeploy/sync. Claims are intentionally retained for as long as the signing
// key can validate a captured body; an age-based purge would reopen the replay
// path. See migrations/0074 for the table contract and 0092 for the historical
// created_at index (codex round-16 #12).

// ClaimGitWebhookDelivery atomically claims a delivery-body digest. It returns
// true when this caller is the first to claim (the delivery should proceed) and
// false when the body was already processed (a replay: answer success, mutate
// nothing). The INSERT .. ON CONFLICT DO NOTHING is the atomicity boundary —
// concurrent duplicates of one delivery collapse to exactly one mutation.
func (s *PGStore) ClaimGitWebhookDelivery(ctx context.Context, digest string) (bool, error) {
	const q = `INSERT INTO git_webhook_replays (digest) VALUES ($1) ON CONFLICT (digest) DO NOTHING`
	tag, err := s.Pool.Exec(ctx, q, digest)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ReleaseGitWebhookDelivery frees a claim whose delivery failed before
// completing (the webhook answered 5xx and the git host will redeliver): the
// retry must not be swallowed by a claim whose work never happened. A delivery
// that completed — even with per-app failures, which the webhook deliberately
// 200-swallows — keeps its claim: that IS the processed state.
func (s *PGStore) ReleaseGitWebhookDelivery(ctx context.Context, digest string) error {
	const q = `DELETE FROM git_webhook_replays WHERE digest = $1`
	_, err := s.Pool.Exec(ctx, q, digest)
	return err
}
