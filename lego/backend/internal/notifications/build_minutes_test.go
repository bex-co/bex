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

package notifications

import (
	"strings"
	"testing"
)

// w6/m308: the build-minutes emails say what happens next, which depends on
// whether the workspace can pay for minutes beyond its plan's.
func TestBuildMinutesEmailSaysWhatHappensNext(t *testing.T) {
	for _, tc := range []struct {
		stage   string
		canPay  bool
		want    string
		notWant string
	}{
		{"warned", false, "stop until the start of next month", "billed per minute"},
		{"warned", true, "billed per minute", "stop"},
		{"exhausted", false, "Add a payment method to keep building", "billed per minute"},
		{"exhausted", true, "billed per minute", "stop"},
	} {
		subject, msg, err := buildMinutesEmail(tc.stage, 450, 500, tc.canPay, "https://dashboard.example")
		if err != nil {
			t.Fatalf("%s/%v: %v", tc.stage, tc.canPay, err)
		}
		body := strings.Join(msg.Paragraphs, " ")
		if !strings.Contains(body, tc.want) || strings.Contains(body, tc.notWant) {
			t.Errorf("%s/canPay=%v body = %q, want %q and not %q", tc.stage, tc.canPay, body, tc.want, tc.notWant)
		}
		if !strings.HasPrefix(subject, "[bex] ") || msg.CTA == nil || msg.CTA.URL != "https://dashboard.example/billing" {
			t.Errorf("%s: subject %q / CTA %+v, want a [bex] subject linking to /billing", tc.stage, subject, msg.CTA)
		}
	}
	if _, _, err := buildMinutesEmail("bogus", 0, 500, false, ""); err == nil {
		t.Error("an unknown stage must be an error")
	}
}
