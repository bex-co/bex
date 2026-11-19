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

package api

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// credentialDisposal is the one teardown of a person's machine credentials: the
// API keys (apikeys.AccountTeardown) and, with them, the S3 access keys they made
// (ADR112 D3). A member's removal, a member leaving and an account's deletion all
// reach it, so an S3 key dies with the membership that justified it by the same
// rule an API key does (ADR024 w2/m163, ADR086), and the members audit row counts
// both kinds. Either half may be absent: a deployment with no Hydra has no API
// keys, one with no object storage has no S3 keys.
type credentialDisposal struct {
	apiKeys *apikeys.AccountTeardown
	s3Keys  *objectstorage.AccessKeys
}

// newCredentialDisposal returns nil when there is nothing to dispose, so a caller
// that tests for a nil teardown keeps its meaning.
func newCredentialDisposal(apiKeys *apikeys.AccountTeardown, s3Keys *objectstorage.AccessKeys) *credentialDisposal {
	if apiKeys == nil && s3Keys == nil {
		return nil
	}
	return &credentialDisposal{apiKeys: apiKeys, s3Keys: s3Keys}
}

// CleanupSubjectInWorkspace disposes of the keys subject made in one workspace
// and returns every id it disposed of. The API keys go first and an error from
// them stops the disposal, as it does without S3 keys: the removal must not
// report success over a credential it did not kill.
func (d *credentialDisposal) CleanupSubjectInWorkspace(ctx context.Context, subject, workspaceID string) ([]string, error) {
	var disposed []string
	if d.apiKeys != nil {
		ids, err := d.apiKeys.CleanupSubjectInWorkspace(ctx, subject, workspaceID)
		disposed = append(disposed, ids...)
		if err != nil {
			return disposed, err
		}
	}
	ids, err := d.s3Keys.Dispose(ctx, subject, workspaceID)
	return append(disposed, ids...), err
}

// CleanupSubject disposes of every key subject made, in every workspace (an
// account's deletion).
func (d *credentialDisposal) CleanupSubject(ctx context.Context, subject string) error {
	if d.apiKeys != nil {
		if err := d.apiKeys.CleanupSubject(ctx, subject); err != nil {
			return err
		}
	}
	_, err := d.s3Keys.Dispose(ctx, subject, "")
	return err
}

// List is the account worker's view of the API keys; S3 keys are disposed of by
// subject and never listed here.
func (d *credentialDisposal) List(ctx context.Context) ([]apikeys.APIKey, error) {
	if d.apiKeys == nil {
		return nil, nil
	}
	return d.apiKeys.List(ctx)
}
