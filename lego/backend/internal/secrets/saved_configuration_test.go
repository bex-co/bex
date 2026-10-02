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

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Existing references are the production regression: saving new file bytes
// changes neither the App spec nor its pending-reference annotations. Every
// effective save must nevertheless notify the status reconciler, even if the
// clock is unchanged, while a no-op must not create an event or notification.
func TestSaveOnlyExistingFileNotifiesWithoutRollout(t *testing.T) {
	ctx := auditCtx()
	app := managedApp("web")
	app.Generation = 7
	app.Spec.FilesFromSecrets = []string{"web-files"}
	app.Spec.RestartedAt = "running-release"
	app.Status = appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ActiveRevision: "rev-7"}
	store := newFakeSecretStore()
	store.m[filesPath("web")] = map[string]string{"message": "fixture-v1"}
	svc := newService(store, app, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-files", Namespace: "default"},
		Data:       map[string][]byte{"message": []byte("fixture-v1")},
	})
	counting := &patchCountingClient{Client: svc.Client}
	rec, audit := &recordingDeploys{}, &recordingAuditSink{}
	svc.Client, svc.Audit = counting, audit
	svc.Rollout = &rollout.Tracker{Store: rec}
	prior := ""
	for i, value := range []string{"fixture-v2", "fixture-v3", "fixture-v1"} {
		patch := EnvironmentPatch{SaveMode: SaveModeOnly, SecretFiles: []SecretFilePatch{{Name: "message", Content: value}}}
		result, err := svc.PatchEnvironment(ctx, "web", patch)
		if err != nil {
			t.Fatal(err)
		}
		after := getApp(t, svc.Client, "web")
		notification := after.Annotations[appv1alpha1.AnnotationSavedConfigRevision]
		if notification == "" || notification == prior {
			t.Fatalf("effective save %d did not publish a fresh status notification", i+1)
		}
		if !reflect.DeepEqual(after.Spec, app.Spec) || !reflect.DeepEqual(after.Status, app.Status) || after.Generation != app.Generation || result.RolledOut || len(rec.rows) != 0 {
			t.Fatal("Save only changed runtime intent/status or opened a deploy")
		}
		if string(getSecret(t, svc.Client, "web-files").Data["message"]) != value || store.m[filesPath("web")]["message"] != value {
			t.Fatal("Save only did not persist/project the submitted file")
		}
		if counting.patches != i+1 || len(audit.events) != i+1 {
			t.Fatalf("effective saves patched/audited %d/%d times, want %d", counting.patches, len(audit.events), i+1)
		}
		encoded, err := json.Marshal(struct {
			Annotations map[string]string
			Status      appv1alpha1.AppStatus
			Result      EnvironmentPatchResult
			Audit       []core.AuditEvent
		}{after.Annotations, after.Status, result, audit.events})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "fixture-v") {
			t.Fatal("status notification or public metadata exposed file contents")
		}
		if _, err := svc.PatchEnvironment(ctx, "web", patch); err != nil {
			t.Fatal(err)
		}
		if got := getApp(t, svc.Client, "web").Annotations[appv1alpha1.AnnotationSavedConfigRevision]; got != notification || counting.patches != i+1 || len(audit.events) != i+1 {
			t.Fatal("no-op save created another notification or event")
		}
		prior = notification
	}
}

func TestSaveOnlyNotificationCoversSourceChanges(t *testing.T) {
	for _, serviceType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite} {
		for _, change := range []string{"new references", "existing mixed", "delete all", "CAS existing variable"} {
			t.Run(serviceType+"/"+change, func(t *testing.T) {
				app := sampleApp("web")
				app.Spec.Type = serviceType
				store := newVersionedFakeSecretStore()
				patch := EnvironmentPatch{SaveMode: SaveModeOnly,
					EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "after"}}, SecretFiles: []SecretFilePatch{{Name: "message", Content: "after"}}}
				if change != "new references" {
					app.Spec.EnvFromSecret = "web-env"
					app.Spec.FilesFromSecrets = []string{"web-files"}
					store.m[envPath("web")] = map[string]string{"TOKEN": "before"}
					store.m[filesPath("web")] = map[string]string{"message": "before"}
				}
				if change == "delete all" {
					patch.EnvVars = []EnvVarPatch{{Key: "TOKEN", Delete: true}}
					patch.SecretFiles = []SecretFilePatch{{Name: "message", Delete: true}}
				}
				if change == "CAS existing variable" {
					revision := encodeEnvRevision(0)
					patch.ExpectedEnvRevision, patch.SecretFiles = &revision, nil
				}
				svc := newService(store, app)
				result, err := svc.PatchEnvironment(context.Background(), "web", patch)
				if err != nil {
					t.Fatal(err)
				}
				after := getApp(t, svc.Client, "web")
				if after.Annotations[appv1alpha1.AnnotationSavedConfigRevision] == "" || !reflect.DeepEqual(after.Spec, app.Spec) || result.RolledOut {
					t.Fatal("source change failed to notify without changing runtime intent")
				}
				if change == "CAS existing variable" {
					patch.ExpectedEnvRevision = result.Revision
					noop, err := svc.PatchEnvironment(context.Background(), "web", patch)
					if err != nil {
						t.Fatal(err)
					}
					if noop.Revision == nil || *noop.Revision == *result.Revision || noop.RolledOut || getApp(t, svc.Client, "web").Annotations[appv1alpha1.AnnotationSavedConfigRevision] != after.Annotations[appv1alpha1.AnnotationSavedConfigRevision] {
						t.Fatal("CAS no-op did not advance only the source concurrency revision")
					}
				}
			})
		}
	}
}

func TestSaveOnlyFailedNotificationRestoresPriorSavedConfiguration(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, cas := range []bool{false, true} {
			name := "existing"
			if pending {
				name = "pending"
			}
			if cas {
				name += "/CAS"
			}
			t.Run(name, func(t *testing.T) {
				app := sampleApp("web")
				app.Annotations = map[string]string{appv1alpha1.AnnotationSavedConfigRevision: "prior-notification"}
				app.Status.UndeployedChanges = true
				if pending {
					app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] = "web-env"
					app.Annotations[appv1alpha1.PendingFilesSecretAnnotation] = "web-files"
				} else {
					app.Spec.EnvFromSecret = "web-env"
					app.Spec.FilesFromSecrets = []string{"web-files"}
				}
				store := newVersionedFakeSecretStore()
				store.m[envPath("web")] = map[string]string{"TOKEN": "previously-saved"}
				store.m[filesPath("web")] = map[string]string{"message": "previously-saved"}
				objects := []client.Object{app}
				for _, kind := range []string{"env", "files"} {
					key := "TOKEN"
					if kind == "files" {
						key = "message"
					}
					objects = append(objects, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "web-" + kind, Namespace: "default"},
						Data:       map[string][]byte{key: []byte("previously-saved")},
					})
				}
				svc := newService(store, objects...)
				svc.Client = &patchCountingClient{Client: svc.Client, fail: errors.New("notification unavailable")}
				patch := EnvironmentPatch{SaveMode: SaveModeOnly,
					EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "failed-save"}}, SecretFiles: []SecretFilePatch{{Name: "message", Content: "failed-save"}}}
				if cas {
					revision := encodeEnvRevision(0)
					patch.ExpectedEnvRevision, patch.SecretFiles = &revision, nil
				}
				if _, err := svc.PatchEnvironment(context.Background(), "web", patch); err == nil {
					t.Fatal("failed notification was reported as a successful save")
				} else if cas {
					var coded *core.CodedError
					if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_UPDATE_RESTORED" {
						t.Fatalf("CAS compensation lost its coded disposition: %v", err)
					}
				}
				after := getApp(t, svc.Client, "web")
				if !reflect.DeepEqual(after.Spec, app.Spec) || !reflect.DeepEqual(after.Status, app.Status) || !reflect.DeepEqual(after.Annotations, app.Annotations) {
					t.Fatal("failed save changed previous intent, notification or status")
				}
				if store.m[envPath("web")]["TOKEN"] != "previously-saved" || store.m[filesPath("web")]["message"] != "previously-saved" {
					t.Fatal("failed save lost the previously saved source")
				}
				if string(getSecret(t, svc.Client, "web-env").Data["TOKEN"]) != "previously-saved" || string(getSecret(t, svc.Client, "web-files").Data["message"]) != "previously-saved" {
					t.Fatal("failed save lost a previously saved projection")
				}
			})
		}
	}
}
