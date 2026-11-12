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
	CopiedEnvValues(ctx context.Context, copies []core.EnvVarCopy) (map[string]string, error)
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

// CopiedEnvValues reads each copy's source value, keyed by the receiver's
// key, and writes nothing. A missing optional source is left out.
func (c envCopier) CopiedEnvValues(ctx context.Context, copies []core.EnvVarCopy) (map[string]string, error) {
	values := make(map[string]string, len(copies))
	for _, want := range copies {
		source, _, err := c.service.envVarValue(ctx, want.From, want.FromKey)
		if want.Optional && errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		values[want.ToKey] = source.Value
	}
	return values, nil
}

func (c envCopier) CopyEnvVars(ctx context.Context, to string, copies []core.EnvVarCopy) error {
	values, err := c.CopiedEnvValues(ctx, copies)
	if err != nil {
		return err
	}
	missing := map[string]string{}
	for _, want := range copies {
		value, ok := values[want.ToKey]
		if !ok {
			continue
		}
		current, _, err := c.service.envVarValue(ctx, to, want.ToKey)
		switch {
		case errors.Is(err, core.ErrNotFound):
			missing[want.ToKey] = value
		case err != nil:
			return err
		case current.Value != value:
			if _, err := c.service.SetEnvVar(ctx, to, want.ToKey, EnvVarWrite{Value: value}); err != nil {
				return err
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return c.service.SeedEnvVars(ctx, to, missing, nil)
}
