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

package controller

import (
	"context"
 "encoding/json"
 "os"
 "path/filepath"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
 "k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPreDeployJobRunsInAppNamespace pins the ADR043-D8 co-location fix: even
// with BEX_BUILD_NAMESPACE set, the pre-deploy Job lands in the App's OWN
// namespace — a Job in the shared build namespace cannot reach the workspace's
// managed Postgres through the tenant default-deny, so a migration's
// connections timed out there. Co-location also retires the w9/012 relocation
// machinery this file used to pin: pull secrets are exactly the app pod's
// imagePullSecrets() (no build-namespace credential, no external-secret
// relocation) and no referenced Secret is copied across namespaces.
func TestPreDeployJobRunsInAppNamespace(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)

	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myapp", Namespace: "default", Generation: 1, UID: "uid-myapp",
			Labels: map[string]string{labelWorkspace: "tea-ws"},
		},
		Spec: appv1alpha1.AppSpec{
			Image:                      "zot.svc:5000/myapp:gen-1",
			PreDeployCommand:           "migrate",
			EnvFromSecret:              "myapp-env",
			ExternalRegistryPullSecret: "ext-pull",
		},
	}
	envSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "myapp-env", Namespace: "default"}}
	extPull := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ext-pull", Namespace: "default"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(app, envSecret, extPull).
		WithStatusSubresource(&appv1alpha1.App{}).
		Build()
	r := &AppReconciler{
		Client: cl, BuildClient: cl, Scheme: scheme,
		Mode: ModeKubernetes, BuildNamespace: "bex-build",
		Registry: "zot.svc:5000", PerAppRegistry: &registry.Creds{},
	}

	if _, _, err := r.reconcilePreDeploy(ctx, app, "zot.svc:5000/myapp:gen-1", 3000); err != nil {
		t.Fatalf("reconcilePreDeploy: %v", err)
	}

	// The Job runs beside the App, never in the build namespace.
	jobNN := client.ObjectKey{Namespace: "default", Name: predeploy.JobName("myapp", "gen-1")}
	var job batchv1.Job
	if err := cl.Get(ctx, jobNN, &job); err != nil {
		t.Fatalf("pre-deploy Job not in the App namespace: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "bex-build", Name: jobNN.Name}, &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("a pre-deploy Job exists in the build namespace: %v", err)
	}

	// Pull secrets are the app pod's own — the App-namespace tenant credential
	// plus the external-registry reference, resolved in place. Nothing is
	// relocated into the build namespace.
	got := job.Spec.Template.Spec.ImagePullSecrets
	want := []string{appIdentity(app).PullSecretName(), "ext-pull"}
	if len(got) != len(want) {
		t.Fatalf("job imagePullSecrets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("job imagePullSecrets[%d] = %q, want %q", i, got[i].Name, want[i])
		}
	}
	var buildNS batchv1.JobList
	if err := cl.List(ctx, &buildNS, client.InNamespace("bex-build")); err != nil {
		t.Fatal(err)
	}
	if len(buildNS.Items) != 0 {
		t.Errorf("build namespace holds %d Jobs", len(buildNS.Items))
	}
	var secrets corev1.SecretList
	if err := cl.List(ctx, &secrets, client.InNamespace("bex-build")); err != nil {
		t.Fatal(err)
	}
	if len(secrets.Items) != 0 {
		names := make([]string, 0, len(secrets.Items))
		for _, s := range secrets.Items {
			names = append(names, s.Name)
		}
		t.Errorf("secrets were mirrored into the build namespace: %v — the Job resolves every reference in the App namespace now", names)
	}
}

// Scratch-only exporter: fake App inputs flow through the actual controller.
func TestScratchDev8PredeployTemplate(t *testing.T) {
 dir:=os.Getenv("BEX_DEV8_OUTPUT")
 read:=func(name string) []byte { b,e:=os.ReadFile(filepath.Join(dir,name));if e!=nil {t.Fatal(e)};return b }
 var input struct { Name, Namespace, AnchorUID, Image, PullSecret, EnvSecret, FileSecret string; Generation int64 }
 inputFile:=os.Getenv("BEX_DEV8_INPUT");if inputFile=="" {inputFile="generator-input.json"}
 if e:=json.Unmarshal(read(inputFile),&input);e!=nil {t.Fatal(e)};if input.Generation==0 {input.Generation=1}
 ctx:=context.Background();scheme:=runtime.NewScheme()
 if e:=clientgoscheme.AddToScheme(scheme);e!=nil {t.Fatal(e)}
 if e:=appv1alpha1.AddToScheme(scheme);e!=nil {t.Fatal(e)}
 app:=&appv1alpha1.App{ObjectMeta:metav1.ObjectMeta{Name:input.Name,Namespace:input.Namespace,Generation:input.Generation,UID:types.UID(input.AnchorUID),Labels:map[string]string{labelWorkspace:"dev-8"}},Spec:appv1alpha1.AppSpec{Runtime:"node",Builder:"native",Repo:"https://github.com/bex-co/git-fixtures",Tier:"starter",PreDeployCommand:string(read("command.txt")),EnvFromSecret:input.EnvSecret,FilesFromSecrets:[]string{input.FileSecret},ExternalRegistryPullSecret:input.PullSecret,Env:[]appv1alpha1.EnvVar{{Name:"QA_LITERAL",Value:"literal $(QA_SECRET_MARKER) $HOME; \"雪\"\ntrailing\n"},{Name:"QA_EMPTY",Value:""},{Name:"QA_OVERRIDE",Value:"literal-wins"}}}}
 cl:=fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
 r:=&AppReconciler{Client:cl,BuildClient:cl,Scheme:scheme,Mode:ModeKubernetes,BuildNamespace:"bex-build",Registry:"zot.bex-registry.svc:5000"}
 if _,_,e:=r.reconcilePreDeploy(ctx,app,input.Image,10000);e!=nil {t.Fatal(e)}
 var job batchv1.Job
 if e:=cl.Get(ctx,client.ObjectKey{Namespace:input.Namespace,Name:predeploy.JobName(input.Name,appv1alpha1.BuildRevision(input.Generation))},&job);e!=nil {t.Fatal(e)}
 job.TypeMeta=metav1.TypeMeta{APIVersion:"batch/v1",Kind:"Job"}
 for name,value:=range map[string]any{"canonical-job.json":job,"canonical-app.json":app} {
 b,e:=json.MarshalIndent(value,"","  ");if e!=nil {t.Fatal(e)};if e=os.WriteFile(filepath.Join(dir,os.Getenv("BEX_DEV8_VARIANT")+name),b,0600);e!=nil {t.Fatal(e)}
 }
}
