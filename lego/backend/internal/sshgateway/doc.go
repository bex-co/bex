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

// Package sshgateway is the shared kernel of the isolated SSH gateway
// process, which is deployed separately from bex-api so only this process and
// ServiceAccount receive pods/exec permission. The kernel holds what every
// transport shares: the Executor pods/exec seam (KubeExecutor is the only
// place pods/exec is actually issued), the privacy-bounded Metrics, the
// process-wide SessionLimiter, the exec-ticket NonceGuard, the TargetResolver
// authorization seam, and the terminal helpers. Each transport is its own
// sub-package — nativessh (public-key SSH), webshell (Browser Web Shell
// WebSocket), sandboxsse (sandbox exec over SSE), agentcred (pod-bound git
// credential broker) — plus dbrole (the least-privilege DB grant surface) and
// gatewaytest (shared test fakes). cmd/ssh-gateway wires ONE limiter and ONE
// nonce guard into every transport so session caps and ticket single-use hold
// process-wide, not per feature.
package sshgateway
