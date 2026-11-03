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

package agenttranscript

import "testing"

func TestClipHeadTailKeepsBothEndsAndCountsRemoved(t *testing.T) {
	if got := ClipHeadTail("short", 10); got != "short" {
		t.Fatalf("under budget changed: %q", got)
	}
	got := ClipHeadTail("AAAA"+"xxxxxxxx"+"ZZZZ", 8)
	if got != "AAAA\n\n[bex: 8 chars clipped]\n\nZZZZ" {
		t.Fatalf("got %q", got)
	}
	// Multi-byte characters count as characters and are never split.
	if got := ClipHeadTail("你好吗xxxx你好吗", 6); got != "你好吗\n\n[bex: 4 chars clipped]\n\n你好吗" {
		t.Fatalf("got %q", got)
	}
}
