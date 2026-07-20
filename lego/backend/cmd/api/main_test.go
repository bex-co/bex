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

package main

import "testing"

// requireCPAuth is the w1/m53 fail-closed gate on the internal control-plane
// API (:8091): an empty BEX_CP_TOKEN must abort startup unless the explicit
// local-dev override is set.
func TestRequireCPAuth(t *testing.T) {
	cases := []struct {
		name     string
		token    string
		insecure string
		wantErr  bool
	}{
		{"token set", "s3kret", "", false},
		{"token set, insecure ignored", "s3kret", "1", false},
		{"empty token fails closed", "", "", true},
		{"empty token, insecure!=1 still fails", "", "0", true},
		{"empty token, insecure!=1 word still fails", "", "yes", true},
		{"empty token, insecure=1 override", "", "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireCPAuth(tc.token, tc.insecure)
			if (err != nil) != tc.wantErr {
				t.Fatalf("requireCPAuth(%q,%q) err=%v, wantErr=%v", tc.token, tc.insecure, err, tc.wantErr)
			}
		})
	}
}
