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

import "context"

// OutboundIPs is Render's outboundIps schema (GET /services/{serviceId}/
// outbound-ips, operationId retrieve-service-outbound-ips): the addresses a
// service's outbound traffic originates from. bex runs tenant pods on a
// shared, autoscaled node pool with no NAT gateway, so Type is always
// "shared" and IPs is the pool nodes' current ExternalIP set — per-workspace
// dedicated egress IPs are a recorded non-goal (.pm/DO_NOT_DO.md), so
// DedicatedIPID is never populated (Render emits it only for type=dedicated).
type OutboundIPs struct {
	Type          string   `json:"type"`
	DedicatedIPID string   `json:"dedicatedIpId,omitempty"`
	IPs           []string `json:"ips"`
}

// OutboundIPReader is the seam apps' Service GraphQL type uses to nest the
// outbound-IPs read under a Service; the apps Service satisfies it. Like
// EnvVarReader it lives in the kernel so the shared, stateless Service
// GraphQL type resolves it from the request context the composition root
// injects — no per-server closure.
type OutboundIPReader interface {
	OutboundIPs(ctx context.Context, service string) (OutboundIPs, error)
}

type outboundIPReaderCtxKey struct{}

// WithOutboundIPs returns ctx carrying the outbound-IP reader — the
// composition root's setter, so apps' GraphQL resolver reaches the verb via
// context (see WithEnvVars).
func WithOutboundIPs(ctx context.Context, r OutboundIPReader) context.Context {
	return context.WithValue(ctx, outboundIPReaderCtxKey{}, r)
}

// OutboundIPsFrom returns the outbound-IP reader the root attached (false
// when none is wired — the field then reports ErrUnavailable).
func OutboundIPsFrom(ctx context.Context) (OutboundIPReader, bool) {
	r, ok := ctx.Value(outboundIPReaderCtxKey{}).(OutboundIPReader)
	return r, ok && r != nil
}
