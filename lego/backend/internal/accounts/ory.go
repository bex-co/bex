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

package accounts

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type OryCleaner struct {
	HydraAdminURL  string
	KratosAdminURL string
	Client         *http.Client
}

func NewOryCleaner(hydraAdminURL, kratosAdminURL string) *OryCleaner {
	return &OryCleaner{
		HydraAdminURL:  strings.TrimSuffix(hydraAdminURL, "/"),
		KratosAdminURL: strings.TrimSuffix(kratosAdminURL, "/"),
		Client:         &http.Client{Timeout: 10 * time.Second, Transport: core.OryTransport},
	}
}

func (c *OryCleaner) remove(ctx context.Context, endpoint string) error {
	err := core.DoJSON(ctx, c.Client, http.MethodDelete, endpoint, "", nil, http.StatusNoContent, nil)
	var status *core.HTTPStatusError
	if errors.As(err, &status) && status.Code == http.StatusNotFound {
		return nil
	}
	return err
}

func (c *OryCleaner) CleanupSubject(ctx context.Context, subject string) error {
	if c == nil || c.HydraAdminURL == "" {
		return core.NewUnavailableError(CodeUnavailable, "Hydra account cleanup is not configured", nil)
	}
	encoded := url.QueryEscape(subject)
	// all=true is load-bearing: without it Hydra may revoke only one remembered
	// consent chain instead of every client grant for the subject. Revoking the
	// consent sessions also invalidates their associated OAuth access tokens.
	if err := c.remove(ctx, c.HydraAdminURL+"/admin/oauth2/auth/sessions/consent?subject="+encoded+"&all=true"); err != nil {
		return err
	}
	if err := c.remove(ctx, c.HydraAdminURL+"/admin/oauth2/auth/sessions/login?subject="+encoded); err != nil {
		return err
	}
	return nil
}

func (c *OryCleaner) DeleteSessions(ctx context.Context, subject string) error {
	if c == nil || c.KratosAdminURL == "" {
		return core.NewUnavailableError(CodeUnavailable, "Kratos account cleanup is not configured", nil)
	}
	return c.remove(ctx, c.KratosAdminURL+"/admin/identities/"+url.PathEscape(subject)+"/sessions")
}

func (c *OryCleaner) DeleteIdentity(ctx context.Context, subject string) error {
	if c == nil || c.KratosAdminURL == "" {
		return core.NewUnavailableError(CodeUnavailable, "Kratos account cleanup is not configured", nil)
	}
	return c.remove(ctx, c.KratosAdminURL+"/admin/identities/"+url.PathEscape(subject))
}
