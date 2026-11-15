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

package objectstorage

// Credentials is bex-api's read-write Wasabi identity (ADR112 D9), the only
// object-storage setting that comes from the environment. One IAM user belongs
// to the account, so one pair serves every region.
type Credentials struct {
	AccessKey string // BEX_OBJECT_STORAGE_ACCESS_KEY
	SecretKey string // BEX_OBJECT_STORAGE_SECRET_KEY
}

// Enabled reports whether both halves are set. Without them object storage is
// not offered: the Render routes answer their named 404 and the Cloudflare-shaped
// routes answer 10042, so the code can ship before any bucket exists.
func (c Credentials) Enabled() bool { return c.AccessKey != "" && c.SecretKey != "" }

// Configured reports whether either half is set.
func (c Credentials) Configured() bool { return c.AccessKey != "" || c.SecretKey != "" }

// Missing names the variables still unset when only one half is. Half a
// credential would silently turn the store off, so startup refuses it.
func (c Credentials) Missing() []string {
	if !c.Configured() || c.Enabled() {
		return nil
	}
	if c.AccessKey == "" {
		return []string{"BEX_OBJECT_STORAGE_ACCESS_KEY"}
	}
	return []string{"BEX_OBJECT_STORAGE_SECRET_KEY"}
}

// NewStores builds the Wasabi store of every region from c, or returns nil when
// the credential is not enabled.
func NewStores(c Credentials) map[Region]Store {
	if !c.Enabled() {
		return nil
	}
	stores := make(map[Region]Store, len(Regions))
	for _, r := range Regions {
		stores[r] = NewStore(r.Config(c.AccessKey, c.SecretKey))
	}
	return stores
}
