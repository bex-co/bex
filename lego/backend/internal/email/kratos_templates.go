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

package email

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file generates the KRATOS COURIER templates (verification/recovery
// emails) from the same branded layout every bex-sent transactional email
// uses, so the Ory-sent mail can never drift from the bex look (w6/m42 t013).
// The output is a self-contained Helm values fragment — kratos.emailTemplates
// (the ory/kratos chart mounts each entry under /conf/courier-templates/
// <method>/<result>/email.*.gotmpl) plus the courier template_override_path —
// committed at deploy/gitops/base/values/kratos-email-templates.values.yaml
// and pinned by TestKratosCourierTemplatesInSync. Regenerate with:
//
//	KRATOS_TEMPLATES_PATH=$(git rev-parse --show-toplevel)/deploy/gitops/base/values/kratos-email-templates.values.yaml \
//	  go test ./internal/email/ -run '^TestGenerateKratosCourierTemplates$' -count=1
//
// Sentinels: the layout is html/template, and a literal "{{ .VerificationURL }}"
// in href position would be rejected as #ZgotmplZ. The Message is rendered
// with benign stand-ins (a digits-only token; a well-formed https URL) that
// survive every escaping context unchanged, then the stand-ins are swapped
// for the real Kratos template actions in the rendered string.
const (
	kratosCodeSentinel = "000000111222"
	kratosURLSentinel  = "https://kratos-action-url.invalid/sentinel"
)

// kratosCourierEmail is one courier email: bex's Message shape plus the
// Kratos actions its sentinels expand to and the subject line (a Kratos
// template of its own — it never passes through the HTML layout).
type kratosCourierEmail struct {
	method  string // "verification_code" | "recovery_code"
	result  string // "valid" | "invalid"
	subject string
	message Message
	// codeAction/urlAction replace the sentinels; empty means the message
	// carries no code/URL (the .invalid notices).
	codeAction string
	urlAction  string
}

// kratosCourierEmails is the full override set. The .invalid variants are the
// notices Kratos sends when someone enters an address that belongs to no
// account — branding them too keeps every Ory-sent mail on the bex layout.
// "60 minutes" matches Kratos's default code lifespan, which bex does not
// override; revisit if selfservice.flows.*.lifespan ever changes.
var kratosCourierEmails = []kratosCourierEmail{
	{
		method:  "verification_code",
		result:  "valid",
		subject: "{{ .VerificationCode }} is your bex verification code",
		message: Message{
			Title:      "Verify your email",
			Paragraphs: []string{"Enter this code on the verification page to confirm your email address:"},
			Code: &Code{
				Label: "Verification code",
				Value: kratosCodeSentinel,
				Desc:  "It expires in 60 minutes.",
			},
			CTA: &CTA{
				Lead:  "Or verify with one click",
				Label: "Verify email",
				URL:   kratosURLSentinel,
			},
			Footer: []string{"If this wasn't you, you can safely ignore this email —\nnothing changes without the code."},
		},
		codeAction: "{{ .VerificationCode }}",
		urlAction:  "{{ .VerificationURL }}",
	},
	{
		method:  "verification_code",
		result:  "invalid",
		subject: "Verification attempted with this email address",
		message: Message{
			Title:      "Was this you?",
			Paragraphs: []string{"This email address was entered on bex's verification page, but it isn't attached to any registered account, so nothing was verified."},
			Footer: []string{
				"If this was you, check whether you signed up with a different address.",
				"If it wasn't, you can safely ignore this email.",
			},
		},
	},
	{
		method:  "recovery_code",
		result:  "valid",
		subject: "{{ .RecoveryCode }} is your bex recovery code",
		message: Message{
			Title:      "Recover your account",
			Paragraphs: []string{"Enter this code on the account recovery page to continue:"},
			Code: &Code{
				Label: "Recovery code",
				Value: kratosCodeSentinel,
				Desc:  "It expires in 60 minutes.",
			},
			Footer: []string{"If you didn't request account recovery, you can safely ignore this email —\nyour password was not changed."},
		},
		codeAction: "{{ .RecoveryCode }}",
	},
	{
		method:  "recovery_code",
		result:  "invalid",
		subject: "Account recovery attempted with this email address",
		message: Message{
			Title:      "Was this you?",
			Paragraphs: []string{"This email address was entered on bex's account recovery page, but it isn't attached to any registered account, so no recovery was started."},
			Footer: []string{
				"If this was you, check whether you signed up with a different address.",
				"If it wasn't, you can safely ignore this email.",
			},
		},
	},
}

func (e kratosCourierEmail) render(body string) string {
	if e.codeAction != "" {
		body = strings.ReplaceAll(body, kratosCodeSentinel, e.codeAction)
	}
	if e.urlAction != "" {
		body = strings.ReplaceAll(body, kratosURLSentinel, e.urlAction)
	}
	return body
}

// KratosCourierTemplatesYAML renders the committed Helm values fragment:
// kratos.emailTemplates.<method>.<result>.{subject,body,plainBody} plus the
// courier template_override_path the ory/kratos chart pairs it with.
func KratosCourierTemplatesYAML() ([]byte, error) {
	templates := map[string]map[string]map[string]string{}
	for _, e := range kratosCourierEmails {
		html := e.message.HTML()
		if html == "" {
			return nil, fmt.Errorf("rendering %s/%s HTML body", e.method, e.result)
		}
		if _, ok := templates[e.method]; !ok {
			templates[e.method] = map[string]map[string]string{}
		}
		templates[e.method][e.result] = map[string]string{
			"subject":   e.subject,
			"body":      e.render(html),
			"plainBody": e.render(e.message.Text()),
		}
	}
	values := map[string]any{
		"kratos": map[string]any{
			"emailTemplates": templates,
			"config": map[string]any{
				"courier": map[string]any{
					"template_override_path": "/conf/courier-templates",
				},
			},
		},
	}
	body, err := yaml.Marshal(values)
	if err != nil {
		return nil, err
	}
	header := `# GENERATED FILE — do not edit by hand.
#
# Kratos courier email templates in bex's branded email layout (w6/m42 t013):
# rendered by lego/backend/internal/email (the same layout.html.tmpl + brand
# tokens as every bex-sent transactional email) and pinned by
# TestKratosCourierTemplatesInSync. Regenerate after changing the layout,
# brand tokens, or the courier copy:
#
#   cd lego/backend && KRATOS_TEMPLATES_PATH=$(git rev-parse --show-toplevel)/deploy/gitops/base/values/kratos-email-templates.values.yaml \
#     go test ./internal/email/ -run '^TestGenerateKratosCourierTemplates$' -count=1
#
# The ory/kratos chart mounts each kratos.emailTemplates entry under
# /conf/courier-templates/<method>/<result>/email.*.gotmpl and the
# template_override_path below points Kratos at them; templates not listed
# here fall back to Kratos's defaults.
`
	return append([]byte(header), body...), nil
}
