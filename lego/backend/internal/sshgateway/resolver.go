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

package sshgateway

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

// TargetResolver is the authorization seam the interactive transports
// (nativessh, webshell) re-run themselves: it authorizes the caller's
// identity against the requested App and selects a Ready pod. The production
// implementation is apps.Service.ResolveSSHSession.
type TargetResolver interface {
	ResolveSSHSession(context.Context, string) (apps.SSHInstanceTarget, error)
}
