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
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

const maxAgentCredentialRequest = 16 << 10

type SessionPod struct {
	Name   string
	UID    string
	Labels map[string]string
}

// SessionPodResolver maps the request's direct TCP source IP to exactly one Pod
// in the claimed namespace. The namespace is only a lookup hint: AuthorizePod
// independently checks its exact relationship to the Pod's workspace label.
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

type AgentCredentialConfig struct {
	Pods  SessionPodResolver
	API   *agentsession.Client
	Audit core.AuditSink
	Now   func() time.Time
}

func (s *Server) AgentCredentialEnabled() bool {
	return s.AgentCredential != nil && s.AgentCredential.Pods != nil && s.AgentCredential.API != nil
}

func (s *Server) AgentCredentialHandler() http.Handler {
	return http.HandlerFunc(s.serveAgentCredential)
}

func (s *Server) serveAgentCredential(w http.ResponseWriter, r *http.Request) {
	if !s.AgentCredentialEnabled() {
		http.Error(w, "agent credentials unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	namespace := strings.TrimSpace(r.Header.Get(agentsession.NamespaceHeader))
	sourceIP, err := remoteIP(r.RemoteAddr)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	pod, err := s.AgentCredential.Pods.ResolveSessionPod(r.Context(), namespace, sourceIP)
	if err != nil {
		s.Metrics.authentication("rejected_key")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxAgentCredentialRequest+1))
	if err != nil || len(body) > maxAgentCredentialRequest {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var req agentsession.MintRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	verified, err := agentsession.AuthorizePod(namespace, pod.Name, pod.UID, pod.Labels, req)
	if err != nil {
		s.Metrics.authentication("rejected_target")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	response, err := s.AgentCredential.API.Mint(r.Context(), verified)
	if err != nil {
		if errors.Is(err, agentsession.ErrForbidden) {
			s.Metrics.authentication("rejected_target")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, "credential broker unavailable", http.StatusBadGateway)
		return
	}
	s.Metrics.authentication("accepted")
	core.RecordAuditEvent(r.Context(), s.AgentCredential.Audit, core.AuditEvent{
		Caller: pod.Name, CallerMethod: "sandbox", Verb: agentsession.AuditVerbProxyCredential,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		TargetName: verified.Repository, Outcome: core.AuditAllowed, At: s.agentCredentialNow(),
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) agentCredentialNow() time.Time {
	if s.AgentCredential != nil && s.AgentCredential.Now != nil {
		return s.AgentCredential.Now().UTC()
	}
	return time.Now().UTC()
}

func remoteIP(remoteAddr string) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// Tests and direct listeners may supply a bare IP.
		host = remoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return "", agentsession.ErrForbidden
	}
	return ip.String(), nil
}
