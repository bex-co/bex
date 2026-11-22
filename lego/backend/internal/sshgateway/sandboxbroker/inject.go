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

package sandboxbroker

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

const secretPlaceholder = "${SECRET}"

// RenderTemplate renders a credential injection template by replacing 1-5
// occurrences of `${SECRET}` with secretValue (or returning secretValue
// verbatim when template is empty). Never includes secretValue in errors.
func RenderTemplate(tmpl, secretValue string) (string, error) {
	if secretValue == "" {
		return "", errors.New("credential secret value is empty")
	}
	if tmpl == "" {
		return secretValue, nil
	}
	count := strings.Count(tmpl, secretPlaceholder)
	if count < 1 || count > 5 {
		return "", errors.New("credential template must contain 1 to 5 ${SECRET} placeholders")
	}
	return strings.ReplaceAll(tmpl, secretPlaceholder, secretValue), nil
}

// Inject applies a single matched BrokerCredential to req in place using
// Render's header, basic, cookie, or query injection semantics. Re-validates
// injection settings at injection time as defense in depth (including forbidden
// routing/connection headers). Errors never contain secret bytes.
func Inject(req *http.Request, cred sandbox.BrokerCredential) error {
	if req == nil || req.URL == nil {
		return errors.New("cannot inject credential into nil request")
	}
	// Defense in depth: re-run the canonical Render injection & forbidden-header
	// validator before mutating any outbound request.
	if _, err := sandbox.ValidateCredentialInjection(&cred.Injection, cred.Value); err != nil {
		return errors.New("credential injection configuration failed validation")
	}
	rendered, err := RenderTemplate(cred.Injection.Template, cred.Value)
	if err != nil {
		return err
	}

	switch cred.Injection.Type {
	case sandbox.CredentialInjectionHeader:
		// Replace any caller-supplied value(s) for this header name.
		req.Header.Set(cred.Injection.Name, rendered)
		return nil

	case sandbox.CredentialInjectionBasic:
		encoded := base64.StdEncoding.EncodeToString([]byte(rendered))
		req.Header.Set("Authorization", "Basic "+encoded)
		return nil

	case sandbox.CredentialInjectionCookie:
		injectCookie(req.Header, cred.Injection.Name, rendered)
		return nil

	case sandbox.CredentialInjectionQuery:
		return injectQuery(req.URL, cred.Injection.Name, rendered)

	default:
		return errors.New("unsupported credential injection type")
	}
}

func injectCookie(header http.Header, name, value string) {
	existing := header.Values("Cookie")
	kept := make([]string, 0, len(existing)+1)
	for _, line := range existing {
		for _, pair := range strings.Split(line, ";") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			k, _, ok := strings.Cut(pair, "=")
			if ok && strings.TrimSpace(k) == name {
				continue
			}
			kept = append(kept, pair)
		}
	}
	kept = append(kept, name+"="+value)
	header.Set("Cookie", strings.Join(kept, "; "))
}

func injectQuery(u *url.URL, name, value string) error {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		// Fail closed on malformed query string rather than dropping or
		// mangling parameters; never echo RawQuery or secret.
		return errors.New("failed to parse request query string for credential injection")
	}
	q.Set(name, value)
	u.RawQuery = q.Encode()
	return nil
}
