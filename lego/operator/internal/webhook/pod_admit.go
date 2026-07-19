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

// Package webhook implements the operator's admission webhook handlers.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/bex-co/bex/lego/operator/internal/imagecheck"
)

// PodAdmitter is a validating admission webhook that rejects Pods whose images
// fail cosign signature verification when tenant-image signing is enabled
// (BEX_TENANT_SIGNING_KEY_SECRET set with cosign.pub present, w7/m11).
//
// Only images prefixed with the tenant registry (BEX_REGISTRY) are checked;
// all other images are allowed through unconditionally.
type PodAdmitter struct {
	verifier *imagecheck.Verifier
	registry string // prefix, e.g. "zot.bex-registry.svc:5000"
}

// Handle implements admission.Handler.
func (p *PodAdmitter) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("pod", req.Name, "namespace", req.Namespace)

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		return admission.Errored(http.StatusBadRequest,
			fmt.Errorf("decode pod: %w", err))
	}

	all := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, c := range all {
		if !strings.HasPrefix(c.Image, p.registry) {
			continue // non-tenant image — pass through
		}
		if err := p.verifier.Verify(ctx, c.Image); err != nil {
			logger.Info("rejecting pod: image signature verification failed",
				"container", c.Name, "image", c.Image, "error", err)
			return admission.Denied(
				fmt.Sprintf("image %q failed cosign signature verification: %v", c.Image, err))
		}
		logger.V(1).Info("image signature verified", "container", c.Name, "image", c.Image)
	}
	return admission.Allowed("all tenant images verified")
}

// SetupWithManager reads the cosign public key from secretName in namespace,
// optionally reads registry credentials from pushSecretName (for authenticating
// against the in-cluster Zot registry), and registers the PodAdmitter webhook
// at /validate-v1-pod on the manager's webhook server.
//
// Signing is an explicit activation gate: when secretName is configured the
// public key must exist and the verifier must be ready before the manager can
// become available. Disabled mode does not call this function and its Pods omit
// the webhook's object-selector label.
func SetupWithManager(
	mgr manager.Manager,
	secretName, namespace, registry, pushSecretName string,
) error {
	logger := log.Log.WithName("webhook.pod-admit")

	// Non-cached reader works before the cache starts syncing.
	ar := mgr.GetAPIReader()

	// Fetch the cosign signing Secret.
	var sigSecret corev1.Secret
	if err := ar.Get(context.Background(),
		client.ObjectKey{Name: secretName, Namespace: namespace},
		&sigSecret,
	); err != nil {
		return fmt.Errorf("webhook: get signing secret %s/%s: %w", namespace, secretName, err)
	}

	pubKey, ok := sigSecret.Data["cosign.pub"]
	if !ok || len(pubKey) == 0 {
		return fmt.Errorf("webhook: signing secret %s/%s has no cosign.pub", namespace, secretName)
	}

	// Resolve registry auth from the push Secret (same namespace as signing key).
	auth := ""
	if pushSecretName != "" {
		var pushSecret corev1.Secret
		if err := ar.Get(context.Background(),
			client.ObjectKey{Name: pushSecretName, Namespace: namespace},
			&pushSecret,
		); err != nil {
			logger.Info("push secret not found — registry calls will be unauthenticated",
				"secret", pushSecretName, "error", err)
		} else {
			cfg, cfgOK := pushSecret.Data["config.json"]
			if !cfgOK {
				cfg = pushSecret.Data[".dockerconfigjson"]
			}
			if cfg != nil {
				var err error
				auth, err = imagecheck.RegistryAuth(cfg, registry)
				if err != nil {
					return fmt.Errorf("webhook: parse registry auth from %s/%s: %w",
						namespace, pushSecretName, err)
				}
			}
		}
	}

	// In-cluster Zot is plain HTTP; TLS registries pass scheme="https".
	verifier, err := imagecheck.NewVerifier(pubKey, "http", auth)
	if err != nil {
		return fmt.Errorf("webhook: create verifier from %s/%s: %w", namespace, secretName, err)
	}

	mgr.GetWebhookServer().Register("/validate-v1-pod", &ctrlwebhook.Admission{
		Handler: &PodAdmitter{
			verifier: verifier,
			registry: registry,
		},
	})
	logger.Info("tenant image signature verification enabled",
		"registry", registry, "secret", secretName)
	return nil
}
