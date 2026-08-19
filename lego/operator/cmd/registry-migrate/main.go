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

// registry-migrate copies one App's legacy Zot repository and static S3 prefix
// onto the workspace-scoped identity (w2/m75, docs/ADR074). Dry-run is the
// default; --apply is required to mutate. It never deletes legacy blobs.
//
//	go run ./cmd/registry-migrate --app web --namespace tea-aaa --workspace tea-aaa
//	go run ./cmd/registry-migrate --app web --namespace tea-aaa --workspace tea-aaa --apply
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/identity"
	"github.com/bex-co/bex/lego/operator/internal/migrate"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "registry-migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	appName := flag.String("app", "", "App metadata.name")
	namespace := flag.String("namespace", "", "App namespace")
	workspace := flag.String("workspace", "", "workspace id (tea-…); required if the App has no label")
	all := flag.Bool("all", false, "migrate every App in --namespace")
	apply := flag.Bool("apply", false, "execute the plan (default is dry-run)")
	registryHost := flag.String("registry", envOr("BEX_REGISTRY", "zot.bex-registry.svc:5000"), "Zot host")
	regUser := flag.String("registry-user", envOr("BEX_REGISTRY_USER", "bex-builder"), "registry username for list/digest")
	regPass := flag.String("registry-password", os.Getenv("BEX_REGISTRY_BUILDER_PASSWORD"),
		"registry password (never logged)")
	bucket := flag.String("bucket", os.Getenv("BEX_STATIC_S3_BUCKET"), "static bucket (empty skips S3)")
	endpoint := flag.String("endpoint", os.Getenv("BEX_STATIC_S3_ENDPOINT"), "S3 endpoint")
	flag.Parse()
	if *appName == "" && !*all {
		return fmt.Errorf("provide --app or --all")
	}
	if *namespace == "" {
		return fmt.Errorf("--namespace is required")
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return err
	}
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}
	ctx := context.Background()

	var apps []appv1alpha1.App
	if *all {
		var list appv1alpha1.AppList
		if err := cl.List(ctx, &list, client.InNamespace(*namespace)); err != nil {
			return err
		}
		apps = list.Items
	} else {
		var app appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKey{Namespace: *namespace, Name: *appName}, &app); err != nil {
			return err
		}
		apps = []appv1alpha1.App{app}
	}

	eng := &migrate.Engine{
		Registry: &skopeoRegistry{host: *registryHost, user: *regUser, password: *regPass},
	}
	if *bucket != "" && *endpoint != "" {
		eng.Objects = &awsObjects{bucket: *bucket, endpoint: *endpoint}
	}

	var cluster appv1alpha1.AppList
	if err := cl.List(ctx, &cluster); err != nil {
		return err
	}
	refs := make([]migrate.AppRef, len(cluster.Items))
	for i := range cluster.Items {
		other := &cluster.Items[i]
		refs[i] = migrate.AppRef{
			UID:       string(other.UID),
			Name:      other.Name,
			Workspace: other.Labels["app.bex.co/workspace"],
		}
	}

	for i := range apps {
		app := &apps[i]
		ws := app.Labels["app.bex.co/workspace"]
		if ws == "" {
			ws = *workspace
		}
		if ws == "" {
			return fmt.Errorf("app %s/%s has no workspace label; pass --workspace", app.Namespace, app.Name)
		}
		id := identity.ForApp(app.Name, ws)
		skipTombstone := migrate.SiblingOwnsLegacy(refs, string(app.UID), id)
		plan, err := eng.BuildPlan(ctx, app.Name, app.Namespace, id, skipTombstone)
		if err != nil {
			return fmt.Errorf("%s/%s: %w", app.Namespace, app.Name, err)
		}
		fmt.Print(plan.Format())
		if !*apply {
			fmt.Println("dry-run: pass --apply to execute (no blob deletion either way)")
			continue
		}
		if _, err := eng.Apply(ctx, plan); err != nil {
			return fmt.Errorf("%s/%s: %w", app.Namespace, app.Name, err)
		}
		patch := app.DeepCopy()
		if patch.Labels == nil {
			patch.Labels = map[string]string{}
		}
		patch.Labels["app.bex.co/workspace"] = ws
		if patch.Annotations == nil {
			patch.Annotations = map[string]string{}
		}
		if !skipTombstone {
			patch.Annotations[identity.AnnotTombstone] = identity.TombstoneValue
		}
		if err := cl.Patch(ctx, patch, client.MergeFrom(app)); err != nil {
			return fmt.Errorf("stamp tombstone on %s/%s: %w", app.Namespace, app.Name, err)
		}
		if app.Status.ActiveRevision != "" {
			statusPatch := patch.DeepCopy()
			statusPatch.Status.StaticPrefix = id.StaticPrefix(app.Status.ActiveRevision)
			if err := cl.Status().Patch(ctx, statusPatch, client.MergeFrom(app)); err != nil {
				return fmt.Errorf("stamp staticPrefix on %s/%s: %w", app.Namespace, app.Name, err)
			}
		}
		fmt.Printf("migrated %s/%s\n", app.Namespace, app.Name)
	}
	return nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// skopeoRegistry copies by digest-preserving skopeo copy. List/Digest use the
// distribution HTTP API already in internal/registry.
type skopeoRegistry struct {
	host, user, password string
}

func (s *skopeoRegistry) ListTags(ctx context.Context, repo string) ([]string, error) {
	return registry.ListTags(ctx, nil, s.host, repo, s.user, s.password)
}

func (s *skopeoRegistry) Digest(ctx context.Context, repo, tag string) (string, error) {
	return registry.ResolveDigest(ctx, nil, s.host, repo, tag, s.user, s.password)
}

func (s *skopeoRegistry) CopyTag(ctx context.Context, srcRepo, dstRepo, tag string) error {
	src := fmt.Sprintf("docker://%s/%s:%s", s.host, srcRepo, tag)
	dst := fmt.Sprintf("docker://%s/%s:%s", s.host, dstRepo, tag)
	cmd := exec.CommandContext(ctx, "skopeo", "copy", "--src-tls-verify=false", "--dest-tls-verify=false", src, dst)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func (s *skopeoRegistry) PutTombstone(ctx context.Context, repo, digest string) error {
	src := fmt.Sprintf("docker://%s/%s@%s", s.host, repo, digest)
	dst := fmt.Sprintf("docker://%s/%s:%s", s.host, repo, identity.TombstoneTag)
	cmd := exec.CommandContext(ctx, "skopeo", "copy", "--src-tls-verify=false", "--dest-tls-verify=false", src, dst)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

type awsObjects struct{ bucket, endpoint string }

func (a *awsObjects) List(ctx context.Context, prefix string) ([]migrate.ObjectMeta, error) {
	out, err := exec.CommandContext(ctx, "aws", "s3api", "list-objects-v2",
		"--bucket", a.bucket, "--prefix", prefix, "--endpoint-url", a.endpoint).Output()
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Contents []struct {
			Key  string `json:"Key"`
			ETag string `json:"ETag"`
			Size int64  `json:"Size"`
		} `json:"Contents"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	objs := make([]migrate.ObjectMeta, 0, len(parsed.Contents))
	for _, c := range parsed.Contents {
		objs = append(objs, migrate.ObjectMeta{Key: c.Key, ETag: strings.Trim(c.ETag, `"`), Size: c.Size})
	}
	return objs, nil
}

func (a *awsObjects) Head(ctx context.Context, key string) (migrate.ObjectMeta, error) {
	out, err := exec.CommandContext(ctx, "aws", "s3api", "head-object",
		"--bucket", a.bucket, "--key", key, "--endpoint-url", a.endpoint).Output()
	if err != nil {
		return migrate.ObjectMeta{}, err
	}
	var parsed struct {
		ETag          string `json:"ETag"`
		ContentLength int64  `json:"ContentLength"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return migrate.ObjectMeta{}, err
	}
	return migrate.ObjectMeta{Key: key, ETag: strings.Trim(parsed.ETag, `"`), Size: parsed.ContentLength}, nil
}

func (a *awsObjects) Copy(ctx context.Context, srcKey, dstKey string) error {
	cmd := exec.CommandContext(ctx, "aws", "s3api", "copy-object",
		"--bucket", a.bucket,
		"--key", dstKey,
		"--copy-source", a.bucket+"/"+srcKey,
		"--endpoint-url", a.endpoint)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func (a *awsObjects) PutTombstone(ctx context.Context, key string, body []byte) error {
	cmd := exec.CommandContext(ctx, "aws", "s3api", "put-object",
		"--bucket", a.bucket, "--key", key, "--endpoint-url", a.endpoint, "--body", "-")
	cmd.Stdin = bytes.NewReader(body)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
