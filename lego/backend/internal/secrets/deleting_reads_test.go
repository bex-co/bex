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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/503: the env-var and secret-file reads answer ErrNotFound for a service
// whose deletion is accepted, while a write still reaches it (teardown).
func TestDeletingServiceEnvAndFileReadsAreNotFound(t *testing.T) {
	gone := sampleApp("gone")
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	gone.DeletionTimestamp = &now
	gone.Finalizers = []string{"app.bex.co/finalizer"}
	svc := newService(newFakeSecretStore(), gone, sampleApp("live"))
	ctx := context.Background()

	if _, err := svc.ListEnvVars(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListEnvVars on a deleting service = %v, want ErrNotFound", err)
	}
	if _, err := svc.ListSecretFiles(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListSecretFiles on a deleting service = %v, want ErrNotFound", err)
	}
	if _, err := svc.ListEnvVars(ctx, "live"); err != nil {
		t.Errorf("ListEnvVars on an active service = %v", err)
	}
}
