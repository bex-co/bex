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

package registrycreds

import (
	"context"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/registryread"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CodeImageCheckLimit refuses a check while the workspace already runs as
// many as one replica allows; finishing one clears it.
const CodeImageCheckLimit = "IMAGE_CHECK_LIMIT"

// IsDeployableImage is Render's isDeployableImage (w1/m673): it reads
// imageURL from its registry as the deploy would, with the chosen credential,
// before a service is created from it or switched to it. A member read.
//
// With a credential, the check runs in the credential's workspace, and a
// credential the caller cannot read answers what registryCredential(id)
// answers, never a verdict, so ids cannot be probed. Without one, it runs in
// the caller's workspace. Either way the workspace's concurrent checks are
// bounded.
func (s *Service) IsDeployableImage(ctx context.Context, imageURL, credentialID string) (registryread.Verdict, error) {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID != "" {
		var err error
		if ctx, err = s.scopeCredential(ctx, credentialID); err != nil {
			return registryread.Verdict{}, err
		}
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return registryread.Verdict{}, err
	}
	workspace := s.WorkspaceOrDefault(ctx)
	imageURL = strings.TrimSpace(imageURL)

	var cred *registryread.Credential
	var credHost string
	if credentialID != "" {
		if !s.configured() {
			return registryread.Verdict{}, core.ErrRegistryCredentialsUnavailable
		}
		row, err := s.Store.GetRegistryCredential(ctx, workspace, credentialID)
		if err != nil {
			return registryread.Verdict{}, mapStoreErr(err)
		}
		password, err := s.storedSecret(ctx, workspace, row.ID)
		if err != nil {
			return registryread.Verdict{}, err
		}
		cred = &registryread.Credential{Username: row.Username, Password: password}
		credHost = row.Host
	}

	// What create refuses is refused here first, without a dial.
	if err := store.ValidateImage(imageURL); err != nil {
		return registryread.Refused(registryread.FailureInvalidInputURL, err.Error()), nil
	}
	// The deploy's pull secret names one registry host; another host's image
	// would be pulled anonymously and the credential never used.
	if host := registryHost(imageURL); cred != nil && !sameRegistryHost(credHost, host) {
		return registryread.Refused(registryread.FailureInvalidRegistryCredential,
			"the registry credential is for "+credHost+", not "+host), nil
	}

	checker := s.imageChecker()
	release, ok := checker.Acquire(workspace)
	if !ok {
		return registryread.Verdict{}, core.NewTooManyRequestsError(CodeImageCheckLimit,
			"this workspace is already checking as many images as it may at once; try again in a moment", nil)
	}
	defer release()
	return checker.Check(ctx, imageURL, cred), nil
}

// defaultImageCheck checks linux/amd64 through the dial guard, for a Service
// built without ImageCheck.
var defaultImageCheck = &registryread.Checker{}

func (s *Service) imageChecker() *registryread.Checker {
	if s.ImageCheck != nil {
		return s.ImageCheck
	}
	return defaultImageCheck
}
