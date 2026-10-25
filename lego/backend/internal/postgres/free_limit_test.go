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

package postgres

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func errorCode(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func TestFreePostgresLimit(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "dpg-existing") // plan free

	_, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "second-free", Plan: "free"})
	if !errors.Is(err, core.ErrBadRequest) || errorCode(err) != core.FreeInstanceLimitCode {
		t.Fatalf("second free create => %v, want FREE_INSTANCE_LIMIT", err)
	}
	if _, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "second-free-dry", Plan: "free", DryRun: true}); errorCode(err) != core.FreeInstanceLimitCode {
		t.Fatalf("second free dry-run => %v, want FREE_INSTANCE_LIMIT", err)
	}
	if _, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "paid-one", Plan: "basic-256mb"}); err != nil {
		t.Fatalf("paid create with free present => %v, want admitted", err)
	}
	if _, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "bogus", Plan: "bogus-plan-x", DryRun: true}); err == nil || errorCode(err) == core.FreeInstanceLimitCode {
		t.Fatalf("invalid plan => %v, want plan refusal first", err)
	}
}

func TestFreePostgresLimitTerminatingHoldsSlot(t *testing.T) {
	now := metav1.Now()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-dying", Namespace: "default", DeletionTimestamp: &now, Finalizers: []string{"test/finalizer"}},
		Spec:       appv1alpha1.DatabaseSpec{Name: "dying", Plan: "free"},
	}
	svc, _ := newService(db)
	_, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "another-free", Plan: "free"})
	if err == nil || errorCode(err) != core.FreeInstanceLimitCode {
		t.Fatalf("free create with terminating free present => %v, want FREE_INSTANCE_LIMIT", err)
	}
	if !strings.Contains(err.Error(), "still being deleted") {
		t.Fatalf("terminating refusal should say so: %v", err)
	}
}

func TestFreePostgresConcurrentCreatesAdmitOne(t *testing.T) {
	svc, _ := newService()
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "race-free-" + string(rune('a'+i)), Plan: "free"})
		}()
	}
	wg.Wait()
	admitted := 0
	for _, err := range errs {
		if err == nil {
			admitted++
		}
	}
	if admitted != 1 {
		t.Fatalf("concurrent free creates admitted %d, want 1 (errs=%v)", admitted, errs)
	}
}
