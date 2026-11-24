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
	"slices"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// tenantNodePoolSelector selects the autoscaled tenant node pool
// (infra/clusterapi stamps bex.co/pool=tenant on every worker). It shares
// the canonical key in lego/types (v1alpha1) with the operator's workload
// placement, so the two selects can never drift apart.
var tenantNodePoolSelector = client.MatchingLabels{appv1alpha1.LabelNodePool: appv1alpha1.NodePoolTenant}

// OutboundIPs implements Render's retrieve-service-outbound-ips read
// (GET /v1/services/{id}/outbound-ips → {type, ips, dedicatedIpId?}).
// Authorization and App resolution happen first, against the App's own
// workspace — the same gate as reading the service itself.
//
// With a published egress set configured (OutboundIPSet, ADR125 D2), tenant
// egress leaves through the egress gateway from those addresses, so the answer
// is that fixed set, as Render answers its region's ranges. Without one, a
// pod's egress is masqueraded to the public IP of whichever tenant pool node it
// runs on, so the truthful answer is the pool's current ExternalIP set, which
// changes as nodes join and leave (the divergence ADR018 records). A cluster
// whose pool nodes carry no ExternalIP (the local CAPD mock) reports an empty
// ips array. Type is always "shared": bex has no dedicated egress IP sets (a
// recorded non-goal, .pm/DO_NOT_DO.md), so dedicatedIpId is never present.
func (s *Service) OutboundIPs(ctx context.Context, name string) (core.OutboundIPs, error) {
	if _, err := s.AuthorizeApp(ctx, core.RelCanView, name); err != nil {
		return core.OutboundIPs{}, err
	}
	if len(s.OutboundIPSet) > 0 {
		return core.OutboundIPs{Type: "shared", IPs: slices.Clone(s.OutboundIPSet)}, nil
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
