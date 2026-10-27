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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A service env var no process could start with is refused at write
// (w4/422): bex used to save it and every later deploy failed to exec.
func TestSetEnvVarRefusesAnEntryNoProcessCanStartWith(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	ctx := context.Background()
	fits := strings.Repeat("v", core.MaxEnvEntryBytes-len("BIG=")-1)
	if _, err := svc.SetEnvVar(ctx, "web", "BIG", EnvVarWrite{Value: fits}); err != nil {
		t.Fatalf("an entry at the limit = %v", err)
	}
	var coded *core.CodedError
	if _, err := svc.SetEnvVar(ctx, "web", "BIG", EnvVarWrite{Value: fits + "v"}); !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VARIABLE_TOO_LARGE" {
		t.Fatalf("one byte over = %v, want ENVIRONMENT_VARIABLE_TOO_LARGE", err)
	}
}
