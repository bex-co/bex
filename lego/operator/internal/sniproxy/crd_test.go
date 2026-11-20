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

package sniproxy

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestTheViewsReadTheCRDContract (w6/150): every field a routing view decodes
// is a field of the CRD, at the same JSON path and of the same kind. A routed
// field renamed or retyped in lego/types/v1alpha1 fails here until the view
// follows it, and that change then rolls the proxies, as it must.
func TestTheViewsReadTheCRDContract(t *testing.T) {
	if GroupVersion != appv1alpha1.SchemeGroupVersion {
		t.Errorf("GroupVersion = %v, want the CRDs' %v", GroupVersion, appv1alpha1.SchemeGroupVersion)
	}
	for _, pair := range []struct{ view, crd any }{
		{KeyValue{}, appv1alpha1.KeyValue{}},
		{KeyValueList{}, appv1alpha1.KeyValueList{}},
		{Database{}, appv1alpha1.Database{}},
		{DatabaseList{}, appv1alpha1.DatabaseList{}},
	} {
		view, crd := reflect.TypeOf(pair.view), reflect.TypeOf(pair.crd)
		if view.Name() != crd.Name() {
			t.Errorf("view %s registers kind %q, the CRD's is %q", view, view.Name(), crd.Name())
		}
		for _, problem := range viewMismatches(view.Name(), view, crd) {
			t.Error(problem)
		}
	}
}

// viewMismatches lists each field of view, a struct, that crd lacks at the
// same JSON name or holds with another kind.
func viewMismatches(path string, view, crd reflect.Type) []string {
	var problems []string
	for field := range view.Fields() {
		at := path + "." + cmp.Or(jsonName(field), field.Name)
		got, ok := crdField(crd, field)
		if !ok {
			problems = append(problems, at+": the CRD has no such field")
			continue
		}
		problems = append(problems, typeMismatches(at, field.Type, got.Type)...)
	}
	return problems
}

func typeMismatches(path string, view, crd reflect.Type) []string {
	switch {
	case view == crd:
		return nil
	case view.Kind() != crd.Kind():
		return []string{path + ": the view reads a " + view.Kind().String() + ", the CRD holds a " + crd.Kind().String()}
	case view.Kind() == reflect.Struct:
		return viewMismatches(path, view, crd)
	case view.Kind() == reflect.Slice || view.Kind() == reflect.Pointer:
		return typeMismatches(path+"[]", view.Elem(), crd.Elem())
	}
	return nil
}

func jsonName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	return name
}

func inlined(field reflect.StructField) bool {
	return field.Anonymous && jsonName(field) == ""
}

// crdField finds the field of crd that decodes what the view's field does:
// the same JSON name, looked up inside the structs crd inlines too, or for an
// inlined view field (metav1.TypeMeta) an inlined field of the same type.
func crdField(crd reflect.Type, view reflect.StructField) (reflect.StructField, bool) {
	for field := range crd.Fields() {
		switch {
		case inlined(view):
			if inlined(field) && field.Type == view.Type {
				return field, true
			}
		case !inlined(field):
			if jsonName(field) == jsonName(view) {
				return field, true
			}
		case field.Type.Kind() == reflect.Struct:
			if inner, ok := crdField(field.Type, view); ok {
				return inner, true
			}
		}
	}
	return reflect.StructField{}, false
}

// TestTheViewsDecodeWhatTheAPIServerSends: a CRD object as the API server
// sends it, with fields no proxy reads set too, decodes through the proxy's
// scheme into exactly the routed fields.
func TestTheViewsDecodeWhatTheAPIServerSends(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	decoder := serializer.NewCodecFactory(scheme).UniversalDeserializer()
	decode := func(t *testing.T, sent runtime.Object, into runtime.Object) {
		t.Helper()
		raw, err := json.Marshal(sent)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := decoder.Decode(raw, nil, into); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
	}
	meta := metav1.ObjectMeta{Name: "red-d4abcdefghijklmnopqr", Namespace: "ws-a", Labels: map[string]string{"a": "b"}}
	own := []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "office"}}

	t.Run("KeyValue", func(t *testing.T) {
		var got KeyValue
		decode(t, &appv1alpha1.KeyValue{
			TypeMeta:   metav1.TypeMeta{APIVersion: "app.bex.co/v1alpha1", Kind: "KeyValue"},
			ObjectMeta: meta,
			Spec: appv1alpha1.KeyValueSpec{
				Name: "cache", Plan: "starter", Public: true, InternalAuth: true, MaxmemoryPolicy: "noeviction",
				IPAllowList: own, EnvironmentIPAllowList: []string{"198.51.100.0/24"},
				WorkspaceIPAllowList: []string{"192.0.2.0/24"},
			},
			Status: appv1alpha1.KeyValueStatus{ExternalHost: "red-x.kv.example", Username: "default", Phase: "Running"},
		}, &got)
		want := KeyValue{
			TypeMeta:   metav1.TypeMeta{APIVersion: "app.bex.co/v1alpha1", Kind: "KeyValue"},
			ObjectMeta: meta,
			Spec: KeyValueSpec{
				Plan: "starter", Public: true, IPAllowList: []IPAllowEntry{{CIDR: "203.0.113.0/24"}},
				EnvironmentIPAllowList: []string{"198.51.100.0/24"}, WorkspaceIPAllowList: []string{"192.0.2.0/24"},
			},
			Status: KeyValueStatus{ExternalHost: "red-x.kv.example"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("decoded\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("Database", func(t *testing.T) {
		var got Database
		decode(t, &appv1alpha1.Database{
			TypeMeta:   metav1.TypeMeta{APIVersion: "app.bex.co/v1alpha1", Kind: "Database"},
			ObjectMeta: meta,
			Spec: appv1alpha1.DatabaseSpec{
				Name: "orders", Plan: "basic-256mb", Public: true, Pooler: true, HighAvailability: true,
				ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "east"}},
				Parameters:   map[string]string{"work_mem": "8MB"}, IPAllowList: own,
				EnvironmentIPAllowList: []string{"198.51.100.0/24"}, WorkspaceIPAllowList: []string{"192.0.2.0/24"},
			},
			Status: appv1alpha1.DatabaseStatus{ExternalHost: "dpg-x.db.example"},
		}, &got)
		want := Database{
			TypeMeta:   metav1.TypeMeta{APIVersion: "app.bex.co/v1alpha1", Kind: "Database"},
			ObjectMeta: meta,
			Spec: DatabaseSpec{
				Plan: "basic-256mb", Public: true, Pooler: true, ReadReplicas: []DatabaseReadReplica{{Name: "east"}},
				IPAllowList:            []IPAllowEntry{{CIDR: "203.0.113.0/24"}},
				EnvironmentIPAllowList: []string{"198.51.100.0/24"}, WorkspaceIPAllowList: []string{"192.0.2.0/24"},
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("decoded\n%+v\nwant\n%+v", got, want)
		}
	})
}

// TestTheReplicaLabelsAreTheOperators: pg-sni-proxy names each replica's
// public labels as the operator names the Services behind them, legacy names
// that need a digest included.
func TestTheReplicaLabelsAreTheOperators(t *testing.T) {
	const db = "dpg-d4abcdefghijklmnopqr"
	for _, replica := range []string{
		"a", "east", "reader-a", "Reports_EU", "reports_eu", "1st", "",
		strings.Repeat("x", 63-len(db+"-ro-")), strings.Repeat("x", 40),
	} {
		if got, want := ReadReplicaServiceName(db, replica), appv1alpha1.ReadReplicaServiceName(db, replica); got != want {
			t.Errorf("replica %q: service label %q, the operator's Service is %q", replica, got, want)
		}
		if got, want := ReadReplicaPoolerName(db, replica), appv1alpha1.ReadReplicaPoolerName(db, replica); got != want {
			t.Errorf("replica %q: pooler label %q, the operator's Pooler is %q", replica, got, want)
		}
	}
}

// TestDeepCopyObjectSharesNothing: the informer cache hands each reader a deep
// copy, so a copy must share no slice or map with the cached object.
func TestDeepCopyObjectSharesNothing(t *testing.T) {
	meta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: "x", Labels: map[string]string{"a": "b"}, Finalizers: []string{"f"}}
	}
	kv := KeyValue{ObjectMeta: meta(), Spec: KeyValueSpec{
		IPAllowList: []IPAllowEntry{{CIDR: "203.0.113.0/24"}}, EnvironmentIPAllowList: []string{"198.51.100.0/24"},
		WorkspaceIPAllowList: []string{"192.0.2.0/24"},
	}}
	db := Database{ObjectMeta: meta(), Spec: DatabaseSpec{
		ReadReplicas: []DatabaseReadReplica{{Name: "east"}}, IPAllowList: []IPAllowEntry{{CIDR: "203.0.113.0/24"}},
		EnvironmentIPAllowList: []string{"198.51.100.0/24"}, WorkspaceIPAllowList: []string{"192.0.2.0/24"},
	}}
	scribble := func(m *metav1.ObjectMeta, cidrs []IPAllowEntry, lists ...[]string) {
		m.Labels["a"], m.Finalizers[0], cidrs[0].CIDR = "changed", "changed", "changed"
		for _, list := range lists {
			list[0] = "changed"
		}
	}
	for _, tc := range []struct {
		name     string
		original runtime.Object
		mutate   func(runtime.Object)
	}{
		{"KeyValue", &kv, func(o runtime.Object) {
			c := o.(*KeyValue)
			scribble(&c.ObjectMeta, c.Spec.IPAllowList, c.Spec.EnvironmentIPAllowList, c.Spec.WorkspaceIPAllowList)
		}},
		{"KeyValueList", &KeyValueList{Items: []KeyValue{kv}}, func(o runtime.Object) {
			c := &o.(*KeyValueList).Items[0]
			scribble(&c.ObjectMeta, c.Spec.IPAllowList, c.Spec.EnvironmentIPAllowList, c.Spec.WorkspaceIPAllowList)
		}},
		{"Database", &db, func(o runtime.Object) {
			c := o.(*Database)
			c.Spec.ReadReplicas[0].Name = "changed"
			scribble(&c.ObjectMeta, c.Spec.IPAllowList, c.Spec.EnvironmentIPAllowList, c.Spec.WorkspaceIPAllowList)
		}},
		{"DatabaseList", &DatabaseList{Items: []Database{db}}, func(o runtime.Object) {
			c := &o.(*DatabaseList).Items[0]
			c.Spec.ReadReplicas[0].Name = "changed"
			scribble(&c.ObjectMeta, c.Spec.IPAllowList, c.Spec.EnvironmentIPAllowList, c.Spec.WorkspaceIPAllowList)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := json.Marshal(tc.original)
			if err != nil {
				t.Fatal(err)
			}
			copied := tc.original.DeepCopyObject()
			if !reflect.DeepEqual(copied, tc.original) {
				t.Fatalf("copy %+v differs from %+v", copied, tc.original)
			}
			tc.mutate(copied)
			if after, _ := json.Marshal(tc.original); !bytes.Equal(before, after) {
				t.Errorf("changing the copy changed the original:\n%s\nwas\n%s", after, before)
			}
		})
	}
}

// proxyCommands are the two SNI proxies' packages, from the module root.
var proxyCommands = []string{"./cmd/kv-sni-proxy", "./cmd/pg-sni-proxy"}

// proxyBexPackages are the bex packages the SNI proxies may link. Each one
// rolls both DaemonSets, and closes every external client still open an hour
// later, whenever its code changes (ADR021 § 10): add one only when the
// proxies cannot route without it.
var proxyBexPackages = []string{
	"github.com/bex-co/bex/lego/operator/internal/sniproxy",
	"github.com/bex-co/bex/lego/types/env",
	"github.com/bex-co/bex/lego/types/netutil",
	"github.com/bex-co/bex/lego/types/proxyproto",
	"github.com/bex-co/bex/lego/types/tiers",
}

// TestACRDFieldEditLeavesBothProxiesByteIdentical (w6/150): neither SNI proxy
// links lego/types/v1alpha1, so a field added to the KeyValue and Database
// specs leaves both binaries byte-identical, and the deploy keeps both pins
// (scripts/operator-pins.sh logs "kept").
func TestACRDFieldEditLeavesBothProxiesByteIdentical(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	goTool := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("go", args...)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			stderr := ""
			if exit, ok := err.(*exec.ExitError); ok {
				stderr = string(exit.Stderr)
			}
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, stderr)
		}
		return string(out)
	}

	for _, command := range proxyCommands {
		for dep := range strings.FieldsSeq(goTool("list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", command)) {
			if strings.HasPrefix(dep, "github.com/bex-co/") && !strings.HasPrefix(dep, "github.com/bex-co/bex/lego/operator/cmd/") &&
				!slices.Contains(proxyBexPackages, dep) {
				t.Errorf("%s links %s: each edit there would roll it (ADR021 § 10)", command, dep)
			}
		}
	}

	// The synthetic edit: one field more in each spec, through a build overlay
	// so the tree stays as it is.
	types := strings.TrimSpace(goTool("list", "-f", "{{.Dir}}", "github.com/bex-co/bex/lego/types/v1alpha1"))
	overlay := map[string]string{}
	work := t.TempDir()
	for file, spec := range map[string]string{
		"keyvalue_types.go": "type KeyValueSpec struct {\n",
		"database_types.go": "type DatabaseSpec struct {\n",
	} {
		source, err := os.ReadFile(filepath.Join(types, file))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(source, []byte(spec)) {
			t.Fatalf("%s no longer declares %q; edit another field here", file, strings.TrimSpace(spec))
		}
		edited := bytes.Replace(source, []byte(spec),
			[]byte(spec+"\tSyntheticW6150 string `json:\"syntheticW6150,omitempty\"`\n"), 1)
		replacement := filepath.Join(work, file)
		if err := os.WriteFile(replacement, edited, 0o600); err != nil {
			t.Fatal(err)
		}
		overlay[filepath.Join(types, file)] = replacement
	}
	overlayJSON, err := json.Marshal(map[string]any{"Replace": overlay})
	if err != nil {
		t.Fatal(err)
	}
	overlayFile := filepath.Join(work, "overlay.json")
	if err := os.WriteFile(overlayFile, overlayJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	build := func(dir string, extra ...string) {
		t.Helper()
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		goTool(append(append([]string{"build", "-o", dir + string(filepath.Separator)}, extra...), proxyCommands...)...)
	}
	before, after := filepath.Join(work, "before"), filepath.Join(work, "after")
	build(before)
	build(after, "-overlay", overlayFile)
	for _, command := range proxyCommands {
		name := filepath.Base(command)
		was, is := fileSHA256(t, filepath.Join(before, name)), fileSHA256(t, filepath.Join(after, name))
		if was != is {
			t.Errorf("/%s changed with a field no proxy reads (%x → %x): the deploy would move its pin", name, was, is)
		}
	}
}

func fileSHA256(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}
