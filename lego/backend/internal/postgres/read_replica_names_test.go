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
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// readReplicaNameVectorsPath is the one acceptance table for a read replica
// name: each row's code is what bex-api answers for a new replica of that
// name, null when it accepts it. The dashboard's readReplicaNameProblem is
// tested against the same file
// (dashboard/src/features/databases/lib/__tests__/read-replicas.test.ts), so
// the form and bex-api cannot drift apart (w5/m138).
const readReplicaNameVectorsPath = "testdata/read-replica-name-vectors.json"

func TestReadReplicaNameVectors(t *testing.T) {
	raw, err := os.ReadFile(readReplicaNameVectorsPath)
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name   string  `json:"name"`
		Repeat int     `json:"repeat"`
		Code   *string `json:"code"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	accepted, refused := 0, 0
	for _, v := range vectors {
		name := v.Name
		if v.Repeat > 0 {
			name = strings.Repeat(v.Name, v.Repeat)
		}
		want := ""
		if v.Code != nil {
			want = *v.Code
			refused++
		} else {
			accepted++
		}
		got := ""
		if err := checkReadReplicaNames(nil, []appv1alpha1.DatabaseReadReplica{{Name: name}}); err != nil {
			if got = codedErrorCode(err); got == "" || !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("checkReadReplicaNames(%q) = %v: an uncoded refusal", name, err)
				continue
			}
		}
		if got != want {
			t.Errorf("checkReadReplicaNames(%q) answers code %q, the table says %q", name, got, want)
		}
	}
	if accepted < 5 || refused < 8 {
		t.Fatalf("vector table too small: %d accepted, %d refused", accepted, refused)
	}
	if readReplicaNameMaxLength != 35 {
		t.Fatalf("readReplicaNameMaxLength = %d; the vectors' 35/36 boundary assumes 63 - len(\"dpg-<20>-ro-\")", readReplicaNameMaxLength)
	}
}
