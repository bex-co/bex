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
package apps

import (
	"encoding/json"
	"testing"
)

// w4/389: Render's customDomain carries the owning server {id, name}.
func TestRenderCustomDomainCarriesServer(t *testing.T) {
	out := toRenderCustomDomain(DomainView{Name: "www.example.com", ServiceID: "srv-abc", ServiceName: "web"})
	body, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Server struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"server"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Server.ID != "srv-abc" || decoded.Server.Name != "web" {
		t.Fatalf("server = %+v in %s", decoded.Server, body)
	}
}
