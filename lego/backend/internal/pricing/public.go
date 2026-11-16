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

import "encoding/json"

// publicSheet is the customer-facing price sheet bex-api serves without a
// credential at GET /pricing, so bex.co (eden-cms-v2) can check that its
// pricing page quotes what bex charges. sheetFile embeds it, so a new
// pricing.yaml section stays private until someone moves it here. The agent
// model section (markup, caps, trust tiers, upstream costs) is withheld.
type publicSheet struct {
	Version         string        `json:"version"`
	Workspace       []planPrice   `json:"workspace"`
	Compute         []tierRate    `json:"compute"`
	Postgres        []tierRate    `json:"postgres"`
	KeyValue        []tierRate    `json:"keyvalue"`
	Bandwidth       bandwidthFile `json:"bandwidth"`
	Build           buildFile     `json:"build"`
	Storage         gbSecondRate  `json:"storage"`
	Disk            gbSecondRate  `json:"disk"`
	Sandbox         sandboxRate   `json:"sandbox"`
	SandboxSnapshot gbSecondRate  `json:"sandboxSnapshot"`
}

// PublicJSON is the public price sheet as JSON, built once by parseSheet.
// Callers must not modify the returned bytes.
func (s *Sheet) PublicJSON() []byte { return s.publicJSON }

func publicSheetJSON(f sheetFile) ([]byte, error) {
	return json.MarshalIndent(f.publicSheet, "", "  ")
}
