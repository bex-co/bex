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

package secrets

import (
	"context"
	"errors"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// EnvCopier writes a service's Blueprint copies of its siblings' generated
// values into its env store (w1/m175).
type EnvCopier interface {
	CopyEnvVars(ctx context.Context, to string, copies []core.EnvVarCopy) error
}

// envCopier is built from existing verbs, so it carries their authorization
// and audit unchanged. EnvVarValue reads each source, and the receiver, as the
// sensitive reveal it is. A key the receiver lacks is seeded with the others
// in one SeedEnvVars write (one roll); a key whose source changed since the
// last sync is re-set; an equal one is left alone, so an unchanged sync writes
// nothing and rolls nothing. SetEnvVar is never called for an equal value: it
// materializes, and so restarts the service, even when the store is unchanged.
type envCopier struct{ service *Service }

// NewEnvCopier wraps Service for apps.Service wiring.
func NewEnvCopier(service *Service) EnvCopier {
	return envCopier{service: service}
}

func (c envCopier) CopyEnvVars(ctx context.Context, to string, copies []core.EnvVarCopy) error {
	missing := map[string]string{}
	for _, want := range copies {
		source, err := c.service.EnvVarValue(ctx, want.From, want.FromKey)
		if err != nil {
			return err
		}
		current, err := c.service.EnvVarValue(ctx, to, want.ToKey)
		switch {
		case errors.Is(err, core.ErrNotFound):
			missing[want.ToKey] = source.Value
		case err != nil:
			return err
		case current.Value != source.Value:
			if _, err := c.service.SetEnvVar(ctx, to, want.ToKey, EnvVarWrite{Value: source.Value}); err != nil {
				return err
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return c.service.SeedEnvVars(ctx, to, missing, nil)
}
