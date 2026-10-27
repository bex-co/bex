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
)

// w5/086: a resource in another workspace answers exactly like a missing one,
// yet a caller that skips genuinely missing rows can still tell it apart.
func TestNotInNamedWorkspaceIsANotFoundThatCanBeToldApart(t *testing.T) {
	if !errors.Is(ErrNotInNamedWorkspace, ErrNotFound) || ErrNotInNamedWorkspace.Error() != ErrNotFound.Error() {
		t.Fatalf("ErrNotInNamedWorkspace = %v, want a not-found with the same text", ErrNotInNamedWorkspace)
	}
	if errors.Is(ErrNotFound, ErrNotInNamedWorkspace) || errors.Is(NotFound("project"), ErrNotInNamedWorkspace) {
		t.Fatal("a plain not-found must not read as another workspace's resource")
	}
	var coded *CodedError
	if !errors.As(ErrNotInNamedWorkspace, &coded) || coded.Code != NotFoundCode {
		t.Fatalf("code = %+v, want %s", coded, NotFoundCode)
	}
}
