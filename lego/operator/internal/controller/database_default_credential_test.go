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
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/m265: a Render credential (ownerPrivileges) joins the owner role so it can
// use every object the owner created, and a tombstoned owner keeps its role
// and objects but loses its login — Render's "deleted" original user.
func TestManagedRolesRotateTheDefaultCredential(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	// "first" was an earlier default, since deleted; "second" and "third" are
	// later rotations. Each joins the owner, every retired login and every
	// earlier credential, so tables a former default created stay reachable.
	users := []appv1alpha1.DatabaseUser{
		{Name: "second", SecretName: "s1", OwnerPrivileges: true},
		{Name: "reporting", SecretName: "s2"},
		{Name: "third", SecretName: "s3", OwnerPrivileges: true},
	}
	roles := managedRoleIndex(t, cnpgClusterSpec(clusterParams{plan: plan, storageGB: gb, dbname: "d", owner: "d_user",
		users: users, deletedUsers: []string{"d_user", "first"}}))

	for name, want := range map[string][]any{
		"second": {"d_user", "first"},
		"third":  {"d_user", "first", "second"},
	} {
		if got, _ := roles[name]["inRoles"].([]any); !slices.Equal(got, want) {
			t.Errorf("%s inRoles = %v, want %v", name, roles[name]["inRoles"], want)
		}
	}
	if _, has := roles["reporting"]["inRoles"]; has {
		t.Errorf("an auxiliary user must not inherit the owner: %v", roles["reporting"])
	}
	if !isRetiredRole(roles["first"]) {
		t.Errorf("retired credential = %v", roles["first"])
	}
	owner := roles["d_user"]
	if owner["ensure"] != "present" || owner["login"] != false || owner["disablePassword"] != true {
		t.Errorf("retired owner = %v, want ensure:present login:false disablePassword:true", owner)
	}
	if got := owner["inRoles"].([]any); len(got) != 1 || got[0] != "pg_monitor" {
		t.Errorf("retired owner inRoles = %v, want [pg_monitor]", got)
	}
}

func TestDefaultConnectionDataSwapsTheCredentialEverywhere(t *testing.T) {
	source := map[string][]byte{
		"username":      []byte("d_user"),
		"user":          []byte("d_user"),
		"password":      []byte("ownerpw"),
		"dbname":        []byte("d"),
		"host":          []byte("dpg-x-rw"),
		"port":          []byte("5432"),
		"uri":           []byte("postgresql://d_user:ownerpw@dpg-x-rw.ws.svc:5432/d"),
		"fqdn-uri":      []byte("postgresql://d_user:ownerpw@dpg-x-rw.ws.svc.cluster.local:5432/d"),
		"jdbc-uri":      []byte("jdbc:postgresql://dpg-x-rw.ws.svc:5432/d?password=ownerpw&user=d_user"),
		"fqdn-jdbc-uri": []byte("jdbc:postgresql://dpg-x-rw.ws.svc.cluster.local:5432/d?password=ownerpw&user=d_user"),
		"pgpass":        []byte("dpg-x-rw:5432:d:d_user:ownerpw\n"),
		"unknown":       []byte("prefix-ownerpw"),
	}
	out, err := defaultConnectionData(source, "rotated", "n:ew/pw")
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range out {
		if strings.Contains(string(value), "ownerpw") || strings.Contains(string(value), "d_user") {
			t.Errorf("%s still carries the owner credential: %q", key, value)
		}
	}
	if _, has := out["unknown"]; has {
		t.Errorf("an unknown key carrying the owner password must be dropped")
	}
	want := map[string]string{
		"username": "rotated",
		"password": "n:ew/pw",
		"dbname":   "d",
		"host":     "dpg-x-rw",
		"uri":      "postgresql://rotated:n%3Aew%2Fpw@dpg-x-rw.ws.svc:5432/d",
		"jdbc-uri": "jdbc:postgresql://dpg-x-rw.ws.svc:5432/d?password=n%3Aew%2Fpw&user=rotated",
		"pgpass":   "dpg-x-rw:5432:d:rotated:n:ew/pw\n",
	}
	for key, w := range want {
		if string(out[key]) != w {
			t.Errorf("%s = %q, want %q", key, out[key], w)
		}
	}
}

func TestDatabaseIDFromSecretName(t *testing.T) {
	for in, want := range map[string]string{
		"dpg-a-app": "dpg-a", "dpg-a-default-app": "dpg-a", "dpg-a-pooler-app": "dpg-a",
	} {
		if got, ok := appv1alpha1.DatabaseIDFromSecretName(in); !ok || got != want {
			t.Errorf("%s → %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := appv1alpha1.DatabaseIDFromSecretName("red-a"); ok {
		t.Error("a Key Value Secret names no Database")
	}
}

// w5/m265: a rotation projects the default user's credentials into
// "<id>-default-app" and status.secretName moves only once that Secret is
// current; clearing the default removes it and returns to CNPG's Secret.
func TestDefaultConnectionSecretFollowsTheRotation(t *testing.T) {
	scheme := datastoreSecretScheme(t)
	const ns = "tea-rotate"
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-r", Namespace: ns}}
	db.Spec.Users = []appv1alpha1.DatabaseUser{{Name: "rotated", SecretName: "dpg-r-user-1", OwnerPrivileges: true}}
	db.Spec.DefaultUser = "rotated"
	db.Status.SecretName = "dpg-r-app"
	app := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dpg-r-app", Namespace: ns}, Data: map[string][]byte{
		"username": []byte("dpg_r_user"), "password": []byte("ownerpw"), "dbname": []byte("d"),
		"uri": []byte("postgresql://dpg_r_user:ownerpw@dpg-r-rw." + ns + ".svc:5432/d"),
	}}
	userSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dpg-r-user-1", Namespace: ns},
		Data: map[string][]byte{"username": []byte("rotated"), "password": []byte("newpw")}}
	uncached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &DatabaseReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Scheme: scheme, SecretClient: uncached}
	ctx := context.Background()

	// The user's Secret is not written yet: not ready, and the status keeps
	// naming the previous Secret instead of falling back mid-rotation.
	current, err := r.reconcileDefaultConnectionSecret(ctx, db)
	if err != nil || current != "" {
		t.Fatalf("missing user Secret: current=%q err=%v", current, err)
	}
	db.Status.SecretName = "dpg-r-default-app" // an earlier rotation's Secret
	r.stampConnectionStatus(db, 1, databaseBackupIntent{}, current)
	if db.Status.SecretName != "dpg-r-default-app" {
		t.Fatalf("status.secretName = %q, want the previous value kept", db.Status.SecretName)
	}

	if err := uncached.Create(ctx, userSecret); err != nil {
		t.Fatal(err)
	}
	current, err = r.reconcileDefaultConnectionSecret(ctx, db)
	if err != nil || current != "dpg-r-default-app" {
		t.Fatalf("current=%q err=%v", current, err)
	}
	r.stampConnectionStatus(db, 1, databaseBackupIntent{}, current)
	if db.Status.SecretName != "dpg-r-default-app" {
		t.Fatalf("status.secretName = %q", db.Status.SecretName)
	}
	var got corev1.Secret
	if err := uncached.Get(ctx, client.ObjectKey{Namespace: ns, Name: "dpg-r-default-app"}, &got); err != nil {
		t.Fatal(err)
	}
	if string(got.Data["username"]) != "rotated" || string(got.Data["password"]) != "newpw" ||
		string(got.Data["uri"]) != "postgresql://rotated:newpw@dpg-r-rw."+ns+".svc:5432/d" {
		t.Fatalf("default Secret = %v", got.Data)
	}

	// The pooler Secret now projects the default credential.
	db.Status.PoolerHost = "dpg-r-pooler." + ns + ".svc"
	if ok, err := r.reconcilePoolerConnectionSecret(ctx, db); err != nil || !ok {
		t.Fatalf("pooler: ok=%v err=%v", ok, err)
	}
	var pooler corev1.Secret
	if err := uncached.Get(ctx, client.ObjectKey{Namespace: ns, Name: "dpg-r-pooler-app"}, &pooler); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pooler.Data["uri"]), "postgresql://rotated:newpw@dpg-r-pooler.") {
		t.Fatalf("pooler uri = %s", pooler.Data["uri"])
	}

	db.Spec.DefaultUser = ""
	current, err = r.reconcileDefaultConnectionSecret(ctx, db)
	if err != nil || current != "dpg-r-app" {
		t.Fatalf("clear: current=%q err=%v", current, err)
	}
	r.stampConnectionStatus(db, 1, databaseBackupIntent{}, current)
	if db.Status.SecretName != "dpg-r-app" {
		t.Fatalf("status.secretName = %q, want CNPG's", db.Status.SecretName)
	}
	if err := uncached.Get(ctx, client.ObjectKey{Namespace: ns, Name: "dpg-r-default-app"}, &got); !apierrors.IsNotFound(err) {
		t.Fatalf("default Secret survived clearing: %v", err)
	}
}
