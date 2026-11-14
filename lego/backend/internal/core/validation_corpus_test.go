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

package core

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/validationcorpus"
)

// The corpus files under testdata/validation pin each rule the dashboard's
// forms mirror, so a form and bex-api cannot disagree on an input without a
// test failing. The dashboard asserts the same files against its own
// validators (dashboard/src/test/__tests__/validation-corpus.test.ts).

// TestLabelCorpus pins the free-form label (a service, project or key name):
// CheckLabel accepting a non-empty label, on the text a form sends. The
// endpoints differ on what they trim themselves (TrimmedLabel does, a service
// rename does not), which is why the form trims first.
func TestLabelCorpus(t *testing.T) {
	t.Parallel()
	validationcorpus.Check(t, "label", func(c validationcorpus.Case) error {
		label, err := CheckLabel(validationcorpus.FormTrim(c.Text()))
		if err == nil && label == "" {
			return errors.New("blank")
		}
		return err
	})
}

// TestCIDRCorpus pins an ipAllowList entry: ValidateCIDRs on the one entry.
func TestCIDRCorpus(t *testing.T) {
	t.Parallel()
	validationcorpus.Check(t, "cidr", func(c validationcorpus.Case) error {
		return ValidateCIDRs([]string{validationcorpus.FormTrim(c.Text())})
	})
}

// TestEnvValueCorpus pins the rules an environment variable a save newly
// supplies passes: no NUL, one KEY=value entry within the process limit, and
// a PORT the operator will serve on.
func TestEnvValueCorpus(t *testing.T) {
	t.Parallel()
	validationcorpus.Check(t, "env-value", func(c validationcorpus.Case) error {
		return CheckEnvValue(validationcorpus.FormTrim(c.Key), c.Text())
	})
}
