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

// Package settingsguard is the Kratos settings-flow web hook of a workspace's
// Google-login requirement (w7/m465, docs/ADR012-auth.md §9b). Render: while
// a member belongs to a workspace that enforces Google login, they "can't
// change their Render account's associated email address" and "can't
// disconnect" their Google account; "First, leave any such workspaces."
// Disconnecting any other provider stays allowed (w7/085).
//
// The change happens in Kratos's own settings flow, which any client can call,
// so hiding a control in the dashboard is not enough. Kratos runs a
// flow-interrupting web_hook (response.parse: true) before it saves a profile
// change or an oidc link or unlink, and this verb answers it: 200 lets the
// change through, a 403 with Kratos's message shape refuses it and Kratos
// shows the message. Kratos treats an unreachable hook as a failed flow, so
// the guard fails closed.
//
// Kratos gives the hook neither the request body nor the identity's
// credentials, so it cannot see which provider an unlink removes. The
// dashboard names it in the settings flow's transient_payload ({"unlink":
// "github"}), which Kratos hands the hook as ctx.flow.transient_payload. An
// unlink that names a provider other than Google is let through; one that
// names Google, or none, is refused. A client that names another provider
// while unlinking Google only disconnects its own Google account, and the
// workspace gate then refuses that member, because no linked Google account
// matches their email.
//
// Like the identity-claims verb it is mounted only on bex-api's
// cluster-internal listener and authenticated by a static bearer
// (BEX_KRATOS_SETTINGS_GUARD_TOKEN) that only Kratos holds.
package settingsguard

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Path is the verb's route on the internal mux. Kratos's hook config names the
// settings method in the query: ?method=profile or ?method=oidc.
const Path = "/internal/kratos/settings-guard"

// The Kratos message ids of the two refusals, in the range Kratos leaves to
// web hooks. The dashboard words them in its own language by id.
const (
	MessageEmailChangeRefused = 4100001
	MessageUnlinkRefused      = 4100002
)

// oidcCallbackPath is where a Kratos oidc settings flow lands when it links a
// provider: the hook then runs on the provider's callback. An unlink is the
// settings flow's own POST, so a hook request from anywhere else is an unlink.
const oidcCallbackPath = "/self-service/methods/oidc/callback/"

// googleProvider is the Kratos oidc provider id of Google
// (scripts/lib/kratos-oidc-fragment.sh).
const googleProvider = "google"

// Workspaces names the workspaces a Kratos identity belongs to that require
// Google login (*store.PGStore).
type Workspaces interface {
	GoogleLoginWorkspaces(ctx context.Context, subject string) ([]string, error)
}

// Identities reads the stored identity a settings change starts from: its
// email trait and whether it has a Google account linked. found=false is an
// identity Kratos does not know; an error means Kratos could not answer.
type Identities interface {
	SettingsFacts(ctx context.Context, subject string) (email string, googleLinked, found bool, err error)
}

// Handler answers Kratos's settings web hook.
type Handler struct {
	// Token is the static bearer Kratos sends, compared constant-time before
	// anything else.
	Token      string
	Workspaces Workspaces
	Identities Identities
}

// Register mounts h on the internal mux only when it has a bearer and both
// lookups. Without them the path is not mounted, and a Kratos configured to
// call it fails its settings flows closed, which is the safe direction.
func Register(mux *http.ServeMux, h *Handler) {
	if h == nil || h.Token == "" || h.Workspaces == nil || h.Identities == nil {
		return
	}
	mux.Handle("POST "+Path, h)
}

// request is the hook body the Kratos Jsonnet template sends
// (scripts/lib/kratos-oidc-fragment.sh): the identity as it would be saved,
// the URL of the request that is changing it, and the provider the client
// says an unlink removes (the flow's transient_payload.unlink; empty when the
// client named none).
type request struct {
	IdentityID string `json:"identity_id"`
	Email      string `json:"email"`
	RequestURL string `json:"request_url"`
	Unlink     string `json:"unlink"`
}

// maxBody bounds the hook body: four short strings.
const maxBody = 16 << 10

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+h.Token)) != 1 {
		core.WriteErrStatus(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	method := r.URL.Query().Get("method")
	var req request
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody || json.Unmarshal(body, &req) != nil || req.IdentityID == "" ||
		(method != "profile" && method != "oidc") {
		core.WriteErrStatus(w, http.StatusBadRequest, "invalid settings hook request")
		return
	}
	refusal, err := h.decide(r.Context(), method, req)
	if err != nil {
		log.Printf("settingsguard: %v", err)
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "settings guard unavailable")
		return
	}
	if refusal == nil {
		core.WriteJSON(w, http.StatusOK, struct{}{})
		return
	}
	core.WriteJSON(w, http.StatusForbidden, refusal)
}

// decide returns the refusal for this change, or nil to let it through.
func (h *Handler) decide(ctx context.Context, method string, req request) (*kratosMessages, error) {
	if method == "oidc" && (isLink(req.RequestURL) || (req.Unlink != "" && req.Unlink != googleProvider)) {
		// Linking an account, or unlinking one that is not Google, leaves the
		// Google account the requirement reads.
		return nil, nil
	}
	names, err := h.Workspaces.GoogleLoginWorkspaces(ctx, req.IdentityID)
	if err != nil || len(names) == 0 {
		return nil, err
	}
	email, googleLinked, found, err := h.Identities.SettingsFacts(ctx, req.IdentityID)
	if err != nil || !found {
		return nil, err
	}
	switch {
	case method == "profile" && !strings.EqualFold(strings.TrimSpace(req.Email), email):
		return refuse("#/traits/email", MessageEmailChangeRefused,
			"You can't change your email address while you belong to a workspace that requires Google login. Leave these workspaces first: "+strings.Join(names, ", ")+".", names), nil
	case method == "oidc" && googleLinked:
		// An unlink of Google, or one that does not say which provider it
		// removes and so may be Google's (ADR012 §9b).
		return refuse("#/", MessageUnlinkRefused,
			"You can't disconnect your Google account while you belong to a workspace that requires Google login. Leave these workspaces first: "+strings.Join(names, ", ")+".", names), nil
	}
	return nil, nil
}

// isLink reports whether the hook runs on an oidc provider's callback,
// wherever Kratos's public base URL mounts it.
func isLink(requestURL string) bool {
	u, err := url.Parse(requestURL)
	return err == nil && strings.Contains(u.Path, oidcCallbackPath)
}

// kratosMessages is the body that interrupts a Kratos flow: messages attached
// to a field (instance_ptr "#/traits/email") or to the whole form ("#/").
type kratosMessages struct {
	Messages []kratosFieldMessages `json:"messages"`
}

type kratosFieldMessages struct {
	InstancePtr string          `json:"instance_ptr"`
	Messages    []kratosMessage `json:"messages"`
}

type kratosMessage struct {
	ID      int            `json:"id"`
	Text    string         `json:"text"`
	Type    string         `json:"type"`
	Context map[string]any `json:"context"`
}

func refuse(pointer string, id int, text string, workspaces []string) *kratosMessages {
	return &kratosMessages{Messages: []kratosFieldMessages{{
		InstancePtr: pointer,
		Messages: []kratosMessage{{
			ID: id, Text: text, Type: "error",
			Context: map[string]any{"workspaces": workspaces},
		}},
	}}}
}
