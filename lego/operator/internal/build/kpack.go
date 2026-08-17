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

package build

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

const (
	kpackAPIVersion           = "kpack.io/v1alpha2"
	kpackClusterBuilder       = "bex"
	kpackReadyCondition       = "Ready"
	kpackSucceededCondition   = "Succeeded"
	kpackServiceAccountPrefix = "bex-kpack-"
	kpackPurposeLabel         = "app.bex.co/kpack-purpose"
	kpackRevisionLabel        = "app.bex.co/build-revision"
)

var (
	kpackImageGVK = schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"}
	kpackBuildGVK = schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Build"}
)

// KpackImage constructs the kpack Image resource for one App generation. It is
// unstructured on purpose: the operator needs only this narrow stable contract,
// avoiding kpack's controller implementation and large Knative dependency graph.
func KpackImage(o Options) *unstructured.Unstructured {
	env := make([]any, 0, len(o.BuildEnv))
	for _, item := range o.BuildEnv {
		// kpack rejects SecretKeyRef in Image env. The controller already
		// supplies literal BP_/BPE_ entries only; keep this boundary defensive.
		if item.Name == "" || item.ValueFrom != nil {
			continue
		}
		env = append(env, map[string]any{"name": item.Name, "value": item.Value})
	}

	build := map[string]any{
		"buildTimeout": int64(buildTimeout / time.Second),
		"nodeSelector": map[string]any{execution.NodePoolLabel: execution.UntrustedNodePool},
		"tolerations": []any{map[string]any{
			"key":      execution.BuildPoolTaintKey,
			"operator": "Equal",
			"value":    execution.BuildPoolTaintValue,
			"effect":   "NoSchedule",
		}},
		"resources": map[string]any{
			"requests": map[string]any{"cpu": buildCPURequest, "memory": buildMemoryRequest},
			"limits":   map[string]any{"cpu": buildCPULimit, "memory": buildMemoryLimit},
		},
	}
	if len(env) > 0 {
		build["env"] = env
	}

	spec := map[string]any{
		"tag":                o.KpackImageRef(),
		"serviceAccountName": kpackServiceAccountName(o),
		"builder": map[string]any{
			"name": kpackClusterBuilder,
			"kind": "ClusterBuilder",
		},
		"source": map[string]any{
			"git": map[string]any{
				"url":      o.Repo,
				"revision": o.Ref,
			},
			"subPath": o.RootDir,
		},
		"failedBuildHistoryLimit":  int64(1),
		"successBuildHistoryLimit": int64(1),
		"imageTaggingStrategy":     "None",
		"cache": map[string]any{
			"registry": map[string]any{"tag": o.KpackImageRef() + "-cache"},
		},
		"build": build,
	}
	if o.SignKeySecret != "" {
		// kpack discovers the cosign key through the Image service account.
		spec["cosign"] = map[string]any{}
	}

	appNamespace := ""
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		appNamespace = o.AppNamespace
	}
	commonLabels := execution.PodLabels(o.Name, o.AppUID, "build", o.Workspace, appNamespace, false)
	labels := make(map[string]any, len(commonLabels)+1)
	for key, value := range commonLabels {
		labels[key] = value
	}
	labels["app.bex.co/build"] = o.Name
	labels[kpackPurposeLabel] = kpackImagePurpose
	labels[kpackRevisionLabel] = kpackRevision(o)

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": kpackAPIVersion,
		"kind":       "Image",
		"metadata": map[string]any{
			"name":      kpackImageName(o),
			"namespace": o.Namespace,
			"labels":    labels,
		},
		"spec": spec,
	}}
}

func buildpack(ctx context.Context, o Options) (Result, error) {
	if err := ensureKpackCredentials(ctx, o); err != nil {
		return Result{}, fmt.Errorf("build: prepare kpack credentials: %w", err)
	}
	image := KpackImage(o)
	key := client.ObjectKeyFromObject(image)
	if err := o.Client.Create(ctx, image); err != nil && !apierrors.IsAlreadyExists(err) {
		return Result{}, fmt.Errorf("build: create kpack image %s: %w", key.Name, err)
	}
	if deleting, err := appDeleting(ctx, o); err != nil {
		return Result{}, err
	} else if deleting {
		return Result{}, ErrAppDeleting
	}

	wctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		if deleting, err := appDeleting(wctx, o); err != nil {
			return Result{}, err
		} else if deleting {
			return Result{}, ErrAppDeleting
		}
		cur := newKpackImage()
		if err := o.Client.Get(wctx, key, cur); err != nil {
			if wctx.Err() != nil {
				return Result{}, fmt.Errorf("build: kpack image %s did not finish within %s", key.Name, buildTimeout)
			}
			return Result{}, fmt.Errorf("build: get kpack image %s: %w", key.Name, err)
		}
		if err := checkKpackArtifact(cur, o, kpackImagePurpose); err != nil {
			return Result{}, fmt.Errorf("build: check kpack image identity %s: %w", key.Name, err)
		}
		condition, found := kpackCondition(cur, kpackReadyCondition)
		if found {
			switch condition.Status {
			case corev1.ConditionTrue:
				latest, _, _ := unstructured.NestedString(cur.Object, "status", "latestImage")
				if latest == "" {
					return Result{}, fmt.Errorf("build: kpack image %s is Ready but status.latestImage is empty", key.Name)
				}
				return Result{Image: canonicalKpackImage(o, latest)}, nil
			case corev1.ConditionFalse:
				return Result{}, fmt.Errorf("build: kpack image %s failed: %s", key.Name, kpackFailureMessage(wctx, o.Client, cur, condition))
			}
		}
		select {
		case <-wctx.Done():
			return Result{}, fmt.Errorf("build: kpack image %s did not finish within %s", key.Name, buildTimeout)
		case <-ticker.C:
		}
	}
}

// canonicalKpackImage maps the HTTP-only in-cluster alias back to the operator's
// canonical registry name. Both names reach the same Zot Service; keeping the
// canonical name in App status preserves kubelet hosts.toml, pull-secret, and
// admission-verifier behavior shared with Dockerfile builds.
func canonicalKpackImage(o Options, image string) string {
	if o.KpackRegistry == "" || o.KpackRegistry == o.Registry {
		return image
	}
	prefix := o.KpackRegistry + "/"
	if !strings.HasPrefix(image, prefix) {
		return image
	}
	return o.Registry + "/" + strings.TrimPrefix(image, prefix)
}

type kpackStatusCondition struct {
	Type    string
	Status  corev1.ConditionStatus
	Reason  string
	Message string
}

func kpackCondition(obj *unstructured.Unstructured, conditionType string) (kpackStatusCondition, bool) {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return kpackStatusCondition{}, false
	}
	for _, raw := range conditions {
		m, ok := raw.(map[string]any)
		if !ok || stringValue(m["type"]) != conditionType {
			continue
		}
		return kpackStatusCondition{
			Type:    conditionType,
			Status:  corev1.ConditionStatus(stringValue(m["status"])),
			Reason:  stringValue(m["reason"]),
			Message: stringValue(m["message"]),
		}, true
	}
	return kpackStatusCondition{}, false
}

func kpackFailureMessage(ctx context.Context, cl client.Client, image *unstructured.Unstructured, ready kpackStatusCondition) string {
	if ref, _, _ := unstructured.NestedString(image.Object, "status", "latestBuildRef"); ref != "" {
		build := newKpackBuild()
		if err := cl.Get(ctx, client.ObjectKey{Namespace: image.GetNamespace(), Name: ref}, build); err == nil {
			if condition, ok := kpackCondition(build, kpackSucceededCondition); ok {
				return conditionMessage(condition)
			}
		}
	}
	return conditionMessage(ready)
}

func conditionMessage(condition kpackStatusCondition) string {
	switch {
	case condition.Reason != "" && condition.Message != "":
		return condition.Reason + ": " + condition.Message
	case condition.Message != "":
		return condition.Message
	case condition.Reason != "":
		return condition.Reason
	default:
		return "unknown build failure"
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func newKpackImage() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(kpackImageGVK)
	return u
}

func newKpackBuild() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(kpackBuildGVK)
	return u
}

func newKpackImageList() *unstructured.UnstructuredList {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(kpackImageGVK.GroupVersion().WithKind("ImageList"))
	return u
}

func newKpackBuildList() *unstructured.UnstructuredList {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(kpackBuildGVK.GroupVersion().WithKind("BuildList"))
	return u
}

const (
	kpackImagePurpose          = "image"
	kpackServiceAccountPurpose = "service-account"
	kpackRegistrySecretPurpose = "registry-credential"
	kpackGitSecretPurpose      = "git-credential"
)

func kpackRevision(o Options) string {
	if o.Revision == "" {
		return defaultRevision
	}
	return strings.ToLower(o.Revision)
}

func kpackArtifactName(o Options, readable, purpose string) string {
	return stableKubernetesName(readable,
		"kpack", strings.ToLower(o.Name), o.AppUID, kpackRevision(o), purpose)
}

func kpackImageName(o Options) string {
	return kpackArtifactName(o, "bld-"+o.Name+"-"+kpackRevision(o), kpackImagePurpose)
}

func kpackServiceAccountName(o Options) string {
	return kpackArtifactName(o, kpackServiceAccountPrefix+o.Name, kpackServiceAccountPurpose)
}

func kpackArtifactLabels(o Options, purpose string) map[string]string {
	labels := buildLabels(o)
	labels[kpackPurposeLabel] = purpose
	labels[kpackRevisionLabel] = kpackRevision(o)
	return labels
}

func checkKpackArtifact(obj metav1.Object, o Options, purpose string) error {
	identity := execution.ArtifactIdentity{Name: o.Name, UID: o.AppUID, Workspace: o.Workspace, Namespace: o.AppNamespace}
	if err := identity.CheckOwner(obj); err != nil {
		return err
	}
	labels := obj.GetLabels()
	if labels[kpackPurposeLabel] != purpose || labels[kpackRevisionLabel] != kpackRevision(o) {
		return fmt.Errorf("artifact %s/%s has mismatched kpack purpose or revision", obj.GetNamespace(), obj.GetName())
	}
	return nil
}

// claimKpackArtifact validates a deterministic name before CreateOrUpdate can
// mutate it. Empty metadata denotes a new object; any persisted or labeled
// object must already belong to the exact App UID, revision, and purpose.
func claimKpackArtifact(obj metav1.Object, o Options, purpose string) error {
	if obj.GetResourceVersion() != "" || obj.GetUID() != "" || len(obj.GetLabels()) > 0 {
		if err := checkKpackArtifact(obj, o, purpose); err != nil {
			return err
		}
	}
	obj.SetLabels(kpackArtifactLabels(o, purpose))
	return nil
}

func ensureKpackCredentials(ctx context.Context, o Options) error {
	var secretRefs []corev1.ObjectReference
	var imagePullSecrets []corev1.LocalObjectReference
	if o.PushSecret != "" {
		name, err := ensureKpackRegistrySecret(ctx, o)
		if err != nil {
			return err
		}
		secretRefs = append(secretRefs, corev1.ObjectReference{Name: name})
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: name})
	}
	if o.CloneSecret != "" {
		name, err := ensureKpackGitSecret(ctx, o)
		if err != nil {
			return err
		}
		secretRefs = append(secretRefs, corev1.ObjectReference{Name: name})
	}
	if o.SignKeySecret != "" {
		secretRefs = append(secretRefs, corev1.ObjectReference{Name: o.SignKeySecret})
	}

	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: kpackServiceAccountName(o), Namespace: o.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, o.Client, sa, func() error {
		if err := claimKpackArtifact(sa, o, kpackServiceAccountPurpose); err != nil {
			return err
		}
		sa.Secrets = secretRefs
		sa.ImagePullSecrets = imagePullSecrets
		sa.AutomountServiceAccountToken = ptr(false)
		return nil
	})
	return err
}

func ensureKpackRegistrySecret(ctx context.Context, o Options) (string, error) {
	var source corev1.Secret
	if err := o.Client.Get(ctx, client.ObjectKey{Namespace: o.Namespace, Name: o.PushSecret}, &source); err != nil {
		return "", fmt.Errorf("get registry push secret %s: %w", o.PushSecret, err)
	}
	data := source.Data[corev1.DockerConfigJsonKey]
	if len(data) == 0 {
		data = source.Data["config.json"]
	}
	if len(data) == 0 {
		return "", fmt.Errorf("registry push secret %s has neither %s nor config.json", o.PushSecret, corev1.DockerConfigJsonKey)
	}
	var config struct {
		Auths map[string]json.RawMessage `json:"auths"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("parse registry push secret %s: %w", o.PushSecret, err)
	}
	if config.Auths == nil {
		config.Auths = map[string]json.RawMessage{}
	}
	registry := o.KpackRegistry
	if registry == "" {
		registry = o.Registry
	}
	if _, exists := config.Auths[registry]; !exists && registry != o.Registry {
		if auth, ok := config.Auths[o.Registry]; ok {
			config.Auths[registry] = auth
		}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	name := kpackArtifactName(o, "bld-"+o.Name+"-kpack-registry", kpackRegistrySecretPurpose)
	dst := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: o.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, o.Client, dst, func() error {
		if err := claimKpackArtifact(dst, o, kpackRegistrySecretPurpose); err != nil {
			return err
		}
		dst.Type = corev1.SecretTypeDockerConfigJson
		dst.Data = map[string][]byte{corev1.DockerConfigJsonKey: data}
		return nil
	})
	return name, err
}

func ensureKpackGitSecret(ctx context.Context, o Options) (string, error) {
	var source corev1.Secret
	if err := o.Client.Get(ctx, client.ObjectKey{Namespace: o.Namespace, Name: o.CloneSecret}, &source); err != nil {
		return "", fmt.Errorf("get clone secret %s: %w", o.CloneSecret, err)
	}
	token := source.Data["token"]
	if len(token) == 0 {
		return "", fmt.Errorf("clone secret %s has no token key", o.CloneSecret)
	}
	// SECURITY (codex #5): the clone token is a GitHub installation token — it
	// must only be presented to github.com, never to the (tenant-mutable) repo
	// origin. kpack annotates the BasicAuth secret with this origin and presents
	// it whenever kpack fetches that host; a non-github.com origin would leak the
	// token to an attacker-controlled server. Reject any origin that is not
	// exactly github.com when a clone token is present.
	origin, err := gitHTTPOrigin(o.Repo)
	if err != nil {
		return "", err
	}
	if origin != "https://github.com" {
		return "", fmt.Errorf("kpack clone token is github-scoped but repo origin is %q; refusing to send token to non-github origin", origin)
	}
	name := kpackArtifactName(o, "bld-"+o.Name+"-kpack-git", kpackGitSecretPurpose)
	dst := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: o.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, o.Client, dst, func() error {
		if err := claimKpackArtifact(dst, o, kpackGitSecretPurpose); err != nil {
			return err
		}
		dst.Annotations = map[string]string{"kpack.io/git": origin}
		dst.Type = corev1.SecretTypeBasicAuth
		dst.Data = map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("x-access-token"),
			corev1.BasicAuthPasswordKey: token,
		}
		return nil
	})
	return name, err
}

func gitHTTPOrigin(repo string) (string, error) {
	u, err := url.Parse(repo)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", fmt.Errorf("kpack token authentication requires an http(s) repository URL, got %q", repo)
	}
	return u.Scheme + "://" + u.Host, nil
}

func buildLabels(o Options) map[string]string {
	appNamespace := ""
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		appNamespace = o.AppNamespace
	}
	labels := execution.PodLabels(o.Name, o.AppUID, "build", o.Workspace, appNamespace, false)
	labels["app.bex.co/build"] = o.Name
	return labels
}

func cancelActiveKpackImages(ctx context.Context, name, appUID, namespace string, cl client.Client) error {
	sel := client.MatchingLabels{"app.bex.co/build": name}
	if appUID != "" { // finding 5: UID-scope (rationale in CancelActiveBuilds)
		sel[execution.LabelAppUID] = appUID
	}
	images := newKpackImageList()
	if err := cl.List(ctx, images, client.InNamespace(namespace), sel); err != nil {
		// The Dockerfile path remains usable before kpack is installed.
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind") {
			return nil
		}
		return fmt.Errorf("list kpack builds for %s: %w", name, err)
	}
	for i := range images.Items {
		image := &images.Items[i]
		if kpackImageTerminal(image) {
			continue
		}
		if err := cl.Delete(ctx, image); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("cancel kpack build %s: %w", image.GetName(), err)
		}
	}
	return nil
}

func activeWorkspaceKpackImages(ctx context.Context, workspace, namespace string, cl client.Client) (int, error) {
	images := newKpackImageList()
	if err := cl.List(ctx, images,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.bex.co/component": "build",
			"app.bex.co/workspace": workspace,
		}); err != nil {
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind") {
			return 0, nil
		}
		return 0, fmt.Errorf("list workspace kpack builds: %w", err)
	}
	active := 0
	for i := range images.Items {
		if !kpackImageTerminal(&images.Items[i]) {
			active++
		}
	}
	return active, nil
}

func kpackImageTerminal(image *unstructured.Unstructured) bool {
	condition, found := kpackCondition(image, kpackReadyCondition)
	return found && (condition.Status == corev1.ConditionTrue || condition.Status == corev1.ConditionFalse)
}
