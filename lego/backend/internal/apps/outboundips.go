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
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// tenantNodePoolSelector selects the autoscaled tenant node pool
// (infra/clusterapi stamps bex.co/pool=tenant on every worker). The label
// literal is duplicated from the operator's execution.NodePoolLabel on
// purpose — backend never imports operator.
var tenantNodePoolSelector = client.MatchingLabels{"bex.co/pool": "tenant"}

// OutboundIPs implements Render's retrieve-service-outbound-ips read
// (GET /v1/services/{id}/outbound-ips → {type, ips, dedicatedIpId?}).
// Authorization and App resolution happen before the Node list, against the
// App's own workspace — the same gate as reading the service itself.
//
// The truthful answer is the tenant pool's current ExternalIP set: there is
// no NAT gateway, so a pod's egress sources from the public IP of whichever
// pool node it runs on. The set moves WITH the autoscaler as nodes join and
// leave — exactly how Render warns its own shared regional IPs drift. A
// cluster whose pool nodes carry no ExternalIP (the local CAPD mock)
// truthfully reports an empty ips array. Type is always "shared": bex has no
// dedicated egress IP sets (a recorded non-goal, .pm/DO_NOT_DO.md), so
// dedicatedIpId is never present.
func (s *Service) OutboundIPs(ctx context.Context, name string) (core.OutboundIPs, error) {
	if _, err := s.AuthorizeApp(ctx, core.RelCanView, name); err != nil {
		return core.OutboundIPs{}, err
	}
	var nodes corev1.NodeList
	if err := s.Client.List(ctx, &nodes, tenantNodePoolSelector); err != nil {
		return core.OutboundIPs{}, err
	}
	seen := make(map[string]bool, len(nodes.Items))
	ips := make([]string, 0, len(nodes.Items))
	for i := range nodes.Items {
		for _, address := range nodes.Items[i].Status.Addresses {
			if address.Type != corev1.NodeExternalIP || address.Address == "" || seen[address.Address] {
				continue
			}
			seen[address.Address] = true
			ips = append(ips, address.Address)
		}
	}
	// Sorted so the wire answer is stable across reads even as the API server's
	// Node list order varies.
	sort.Strings(ips)
	return core.OutboundIPs{Type: "shared", IPs: ips}, nil
}
