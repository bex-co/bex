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

package pricing

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The golden file is GET /pricing's public contract. A pricing.yaml change
// that moves it must also update bex.co (eden-cms-v2): `yarn dev` there
// fetches this document, and its tests check the pricing page against it.
// Regenerate with
// BEX_UPDATE_GOLDEN=1 go test ./internal/pricing -run TestPublicSheetGolden.
const publicGolden = "testdata/public_sheet.json"

func TestPublicSheetGolden(t *testing.T) {
	got := append(bytes.Clone(Default.PublicJSON()), '\n')
	if os.Getenv("BEX_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(publicGolden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(publicGolden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("GET /pricing changed; review it, update bex.co, then regenerate %s with BEX_UPDATE_GOLDEN=1:\n%s", publicGolden, got)
	}
}

func TestPublicSheetWithholdsTheAgentModel(t *testing.T) {
	body := string(Default.PublicJSON())
	for _, private := range []string{"agentModel", "markup", "trustTiers", "upstreamPrices"} {
		if strings.Contains(body, private) {
			t.Errorf("public sheet exposes %s", private)
		}
	}
}

func TestPublicSheetMatchesTheChargedSheet(t *testing.T) {
	var p publicSheet
	if err := json.Unmarshal(Default.PublicJSON(), &p); err != nil {
		t.Fatal(err)
	}
	for _, w := range p.Workspace {
		if usd, ok := Default.WorkspaceUSDPerMonth(w.Plan); !ok || usd != w.USDPerMonth {
			t.Errorf("%s plan %v USD/month, charged %v (listed %v)", w.Plan, w.USDPerMonth, usd, ok)
		}
	}
}
