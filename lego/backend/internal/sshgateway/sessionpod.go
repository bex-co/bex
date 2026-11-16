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
	"net"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

// SessionPod is the sandbox Pod a gateway source IP resolves to. It lives in
// the kernel (not agentcred) so every transport that gates on the direct
// source Pod — the agentcred Git proxy, the model proxy, the egress proxy —
// shares one resolution instead of each importing a sibling transport.
type SessionPod struct {
	Name   string
	UID    string
	Labels map[string]string
}

type SessionPodResolver interface {
	ResolveSessionPod(context.Context, string, string) (SessionPod, error)
}

type KubeSessionPodResolver struct{ Client kubernetes.Interface }

func (r KubeSessionPodResolver) ResolveSessionPod(ctx context.Context, namespace, sourceIP string) (SessionPod, error) {
	if r.Client == nil || namespace == "" || net.ParseIP(sourceIP) == nil {
		return SessionPod{}, agentsession.ErrForbidden
	}
	pods, err := r.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("status.podIP", sourceIP).String(),
		Limit:         2,
	})
	if err != nil || len(pods.Items) != 1 {
		return SessionPod{}, agentsession.ErrForbidden
	}
	pod := pods.Items[0]
	if pod.Status.PodIP != sourceIP || pod.DeletionTimestamp != nil {
		return SessionPod{}, agentsession.ErrForbidden
	}
	return SessionPod{Name: pod.Name, UID: string(pod.UID), Labels: pod.Labels}, nil
}

// RemoteIP extracts the validated source IP from a request RemoteAddr. The
// gateway transports (ADR062) resolve the same source Pod from it, so the
// helper lives in the kernel with the resolver.
func RemoteIP(remoteAddr string) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return "", agentsession.ErrForbidden
	}
	return ip.String(), nil
}
