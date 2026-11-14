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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/validationcorpus"
)

// TestCustomDomainCorpus pins a name claimed as a custom domain (see
// core/validation_corpus_test.go): claimableHostname, which every claim passes.
func TestCustomDomainCorpus(t *testing.T) {
	t.Parallel()
	validationcorpus.Check(t, "custom-domain", func(c validationcorpus.Case) error {
		_, err := claimableHostname(c.Text())
		return err
	})
}
