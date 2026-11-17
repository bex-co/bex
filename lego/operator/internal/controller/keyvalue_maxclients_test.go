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
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestValkeyArgsCarryEveryPlansClientLimit(t *testing.T) {
	for _, id := range tiers.Valkey.IDs() {
		plan, _ := tiers.Valkey.ByID(id)
		args := valkeyArgs(appv1alpha1.KeyValueSpec{}, plan)
		want := strconv.Itoa(int(plan.ConnectionLimit) + kvPlatformClientReserve)
		if got := valkeyArgValue([]corev1.Container{{Name: keyValueContainerName, Args: args}}, "--maxclients"); got != want {
			t.Errorf("plan %q: --maxclients %q, want the plan's %d plus the platform's %d", id, got, plan.ConnectionLimit, kvPlatformClientReserve)
		}
	}
}

// A template keeps the limit its server started with only where CONFIG SET can
// reach the plan's from it: lowering, including from the 10,000 a server
// started without the flag runs.
func TestValkeyMaxclientsHoldable(t *testing.T) {
	for _, c := range []struct {
		held, want string
		holdable   bool
	}{
		{"", "60", true},      // a pre-m456 server lowered to free's
		{"", "10000", true},   // exactly the default
		{"", "10010", false},  // pro_plus: past what that server can open
		{"260", "60", true},   // lowered
		{"60", "260", false},  // raised: the server must restart with it
		{"60", "60", false},   // nothing to hold
		{"lots", "60", false}, // unreadable: write the plan's
		{"260", "not-a-number", false},
	} {
		if got := valkeyMaxclientsHoldable(c.held, c.want); got != c.holdable {
			t.Errorf("held %q want %q: holdable %v, want %v", c.held, c.want, got, c.holdable)
		}
	}
}

// t003: the operator upgrade that adds --maxclients rolls no existing store
// whose server CONFIG SET can bring to the plan's limit; a change that rolls
// anyway, a store whose limit only a restart can raise, and a suspended or new
// store all take the plan's flag.
func TestKeyValueTemplateHoldsItsClientLimit(t *testing.T) {
	template := func(plan string, edit func(*appv1alpha1.KeyValueSpec), from *appsv1.StatefulSet) *appsv1.StatefulSet {
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "hold-kv", Namespace: "default"}, Spec: appv1alpha1.KeyValueSpec{Plan: plan}}
		if edit != nil {
			edit(&kv.Spec)
		}
		tier, storage := resolveKVPlan(kv.Spec)
		sts := from.DeepCopy()
		applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, tier, storage, "", false, true))
		return sts
	}
	preM456 := func(plan string) *appsv1.StatefulSet {
		sts := template(plan, nil, &appsv1.StatefulSet{})
		dropValkeyMaxclients(sts)
		sts.ResourceVersion = "7"
		return sts
	}
	limit := func(sts *appsv1.StatefulSet) string {
		return valkeyArgValue(sts.Spec.Template.Spec.Containers, "--maxclients")
	}

	for _, plan := range []string{"free", "starter", "pro"} {
		old := preM456(plan)
		if got := template(plan, nil, old); !equality.Semantic.DeepEqual(got.Spec.Template, old.Spec.Template) || limit(got) != "" {
			t.Errorf("%s: the upgrade rewrote a pre-m456 template (limit %q)", plan, limit(got))
		}
	}
	if got := template("pro_plus", nil, preM456("pro_plus")); limit(got) != "10010" {
		t.Errorf("pro_plus: a server started at 10,000 cannot be raised at runtime, so its template must roll to 10010, got %q", limit(got))
	}
	if got := template("free", func(s *appv1alpha1.KeyValueSpec) { s.Version = "7" }, preM456("free")); limit(got) != "60" {
		t.Errorf("a change that rolls anyway must start the new server with free's 60, got %q", limit(got))
	}
	if got := template("free", func(s *appv1alpha1.KeyValueSpec) { s.Suspended = true }, preM456("free")); limit(got) != "60" {
		t.Errorf("a suspended store's template must resume with 60, got %q", limit(got))
	}
	if got := template("free", nil, &appsv1.StatefulSet{}); limit(got) != "60" {
		t.Errorf("a new store must start with 60, got %q", limit(got))
	}
	// The policy and the limit hold together: a policy edit on a pre-m456
	// template rolls neither.
	old := preM456("free")
	if got := template("free", func(s *appv1alpha1.KeyValueSpec) { s.MaxmemoryPolicy = "allkeys-lru" }, old); !equality.Semantic.DeepEqual(got.Spec.Template, old.Spec.Template) {
		t.Errorf("a policy edit on a pre-m456 template rolled it: limit %q policy %q", limit(got), valkeyArgsMaxmemoryPolicy(got.Spec.Template.Spec.Containers))
	}
}

// dropValkeyMaxclients makes sts's template one a pre-m456 operator wrote:
// its server starts without --maxclients.
func dropValkeyMaxclients(sts *appsv1.StatefulSet) {
	for i, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == keyValueContainerName {
			sts.Spec.Template.Spec.Containers[i].Args = withValkeyArg(c.Args, "--maxclients", "")
		}
	}
}

// t006: on production the operator upgrade that added the limit rolled the two
// oldest stores. One still waited to adopt the datastore priority class, the
// other its environment label, and each hold read the other's pending change
// as a reason to roll. Every change that rolls nothing holds together, and a
// change that rolls anyway still brings all of them.
func TestKeyValueClientLimitHoldsBesideTheOtherDeferredChanges(t *testing.T) {
	for name, pending := range map[string]func(*appv1alpha1.KeyValue, *appsv1.StatefulSet){
		"priority class":    func(_ *appv1alpha1.KeyValue, sts *appsv1.StatefulSet) { sts.Spec.Template.Spec.PriorityClassName = "" },
		"environment label": func(kv *appv1alpha1.KeyValue, _ *appsv1.StatefulSet) { kv.Labels[labelEnvironment] = "env-b" },
		"isolation label":   func(kv *appv1alpha1.KeyValue, _ *appsv1.StatefulSet) { kv.Labels[labelNetworkIsolation] = "env-a" },
		"all three": func(kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet) {
			sts.Spec.Template.Spec.PriorityClassName = ""
			kv.Labels[labelEnvironment] = "env-b"
			kv.Labels[labelNetworkIsolation] = "env-b"
		},
	} {
		kv := isolatedKV("")
		kv.Spec.Plan = "starter"
		apply := func(sts *appsv1.StatefulSet) {
			plan, storage := resolveKVPlan(kv.Spec)
			applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "", false, true))
		}
		sts := &appsv1.StatefulSet{}
		apply(sts)
		sts.CreationTimestamp = metav1.Now()
		dropValkeyMaxclients(sts)
		pending(kv, sts)
		old := sts.Spec.Template.DeepCopy()

		apply(sts)
		if !equality.Semantic.DeepEqual(sts.Spec.Template, *old) {
			t.Errorf("%s: the upgrade rolled a pre-m456 store waiting on it: limit %q, class %q, labels %v", name,
				valkeyArgValue(sts.Spec.Template.Spec.Containers, "--maxclients"), sts.Spec.Template.Spec.PriorityClassName, sts.Spec.Template.Labels)
			continue
		}
		kv.Spec.Plan = "standard"
		apply(sts)
		if got := valkeyArgValue(sts.Spec.Template.Spec.Containers, "--maxclients"); got != "1010" ||
			sts.Spec.Template.Spec.PriorityClassName != appv1alpha1.DatastorePriorityClassName ||
			!inPlaceLabelsEqual(sts.Spec.Template.Labels, kv.Labels) {
			t.Errorf("%s: a plan change rolls anyway but left limit %q, class %q, labels %v", name, got, sts.Spec.Template.Spec.PriorityClassName, sts.Spec.Template.Labels)
		}
	}
}

// t004: a running pre-m456 store adopts its plan's limit with CONFIG SET and
// no roll; a settled store is not dialed again until its container restarts
// from the held template; a refused change is retried soon.
func TestKeyValueMaxclientsReachesRunningServer(t *testing.T) {
	f := newRuntimeTLSFixture(t)
	var calls []string
	var fail error
	f.r.setMaxclients = func(_ context.Context, pod *corev1.Pod, platform *corev1.Secret, maxclients string) error {
		if pod.Status.PodIP != "10.0.0.7" || platform.Name != "cache-kv-platform" {
			t.Fatalf("CONFIG SET dialed pod %q as %q", pod.Status.PodIP, platform.Name)
		}
		if fail != nil {
			return fail
		}
		calls = append(calls, maxclients)
		return nil
	}
	f.reconcileTwice()
	f.startPod()
	f.settleRollout()
	if len(calls) != 0 {
		t.Fatalf("a server started with its plan's limit was dialed: %v", calls)
	}

	// The store as the operator upgrade finds it: started without the flag.
	sts := f.statefulSet()
	dropValkeyMaxclients(sts)
	if err := f.cl.Update(f.ctx, sts); err != nil {
		t.Fatal(err)
	}
	f.settleRollout()
	f.unrolled("the operator upgrade")
	if got := valkeyArgValue(f.template.Spec.Containers, "--maxclients"); got != "" {
		t.Fatalf("the upgrade wrote --maxclients %q into a running store's template", got)
	}
	// annotation is the StatefulSet's record and the record that names the
	// pod's running process with limit. The pod itself is never annotated: the
	// bex-operator-pod-labels admission policy refuses it.
	annotation := func(limit string) (got, want string) {
		pod := &corev1.Pod{}
		if err := f.cl.Get(f.ctx, client.ObjectKey{Namespace: "default", Name: "cache-kv-0"}, pod); err != nil {
			t.Fatal(err)
		}
		if _, ok := pod.Annotations[annotKVMaxclients]; ok {
			t.Fatalf("the record was written on the pod: %v", pod.Annotations)
		}
		return f.statefulSet().Annotations[annotKVMaxclients], limit + "/" + persistenceProcess(pod)
	}
	if got, want := annotation("60"); !slices.Equal(calls, []string{"60"}) || got != want {
		t.Fatalf("CONFIG SET calls %v, record %q; want free's 60 once, recorded as %q", calls, got, want)
	}

	f.reconcileTwice()
	f.unrolled("a settled pass")
	if len(calls) != 1 {
		t.Fatalf("a settled store was dialed again: %v", calls)
	}

	restart := func(n int32) {
		t.Helper()
		pod := &corev1.Pod{}
		if err := f.cl.Get(f.ctx, client.ObjectKey{Namespace: "default", Name: "cache-kv-0"}, pod); err != nil {
			t.Fatal(err)
		}
		pod.Status.ContainerStatuses[0].RestartCount = n
		if err := f.cl.Status().Update(f.ctx, pod); err != nil {
			t.Fatal(err)
		}
		f.reconcileTwice()
	}
	before, _ := annotation("60")
	restart(1)
	if got, want := annotation("60"); !slices.Equal(calls, []string{"60", "60"}) || got != want || got == before {
		t.Fatalf("after a container restart: calls %v, record %q, want %q", calls, got, want)
	}

	fail = errors.New("refused")
	before, _ = annotation("60")
	restart(2)
	if got, _ := annotation("60"); f.result.RequeueAfter != kvRuntimePendingRequeue || got != before {
		t.Fatalf("a refused change: requeue %v, record %q; want a %v retry and the old record", f.result.RequeueAfter, got, kvRuntimePendingRequeue)
	}
	fail = nil
	f.reconcileTwice()
	if got, want := annotation("60"); got != want {
		t.Fatalf("the retry recorded %q, want %q", got, want)
	}
	f.unrolled("the limit's whole runtime life")
}

// TestValkeyRuntimeMaxclients drives the operator's real CONFIG SET against
// both pinned Valkey images started with the pod's own args
// (BEX_TEST_VALKEY=1): the server starts with free's limit and refuses the
// next client with Valkey's error; raising the limit to starter's admits more
// without restarting the process; lowering it below the clients connected
// keeps every one of them and refuses the next; the tenant may not set it.
func TestValkeyRuntimeMaxclients(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker maxclients tests")
	}
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) {
			e := startTLSSwitchEngine(t, major)
			birth := e.runID()
			addr := e.hostPort("6379")
			free, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free"})
			starter, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "starter"})
			if got, _ := e.platform.ConfigGet(e.ctx, "maxclients").Result(); got["maxclients"] != valkeyMaxclients(free) {
				t.Fatalf("started with maxclients %v, want %s", got, valkeyMaxclients(free))
			}
			var held []net.Conn
			t.Cleanup(func() {
				for _, c := range held {
					_ = c.Close()
				}
			})
			// fill opens connections until the server refuses one, and returns
			// how many it admitted and the refusal it read.
			fill := func() (int, string) {
				t.Helper()
				for admitted := range 1000 {
					c, reply := dialPing(t, addr)
					if reply != "-NOAUTH Authentication required." {
						_ = c.Close()
						return admitted, reply
					}
					held = append(held, c)
				}
				t.Fatal("the server never refused")
				return 0, ""
			}
			_, refusal := fill()
			if refusal != "-ERR max number of clients reached" {
				t.Fatalf("at free's limit the next client read %q", refusal)
			}

			if err := setValkeyMaxclients(e.ctx, e.platform, valkeyMaxclients(starter)); err != nil {
				t.Fatalf("raise to starter's: %v", err)
			}
			if more, refusal := fill(); more != int(starter.ConnectionLimit-free.ConnectionLimit) || refusal != "-ERR max number of clients reached" {
				t.Fatalf("after the raise %d more were admitted before %q, want %d", more, refusal, starter.ConnectionLimit-free.ConnectionLimit)
			}
			if e.runID() != birth {
				t.Fatal("the raise restarted the process")
			}

			if err := setValkeyMaxclients(e.ctx, e.platform, valkeyMaxclients(free)); err != nil {
				t.Fatalf("lower below the connected clients: %v", err)
			}
			for i, c := range held {
				if reply := ping(t, c); reply != "-NOAUTH Authentication required." {
					t.Fatalf("held client %d lost by the lower limit: %q", i, reply)
				}
			}
			if _, refusal := fill(); refusal != "-ERR max number of clients reached" {
				t.Fatalf("over the lowered limit the next client read %q", refusal)
			}
			if err := e.tenant.ConfigSet(e.ctx, "maxclients", "100000").Err(); err == nil {
				t.Fatal("the tenant's default user ran CONFIG SET maxclients")
			}
		})
	}
}

// dialPing opens a raw client connection and returns it with the first line
// the server sends after a PING: a refused client reads Valkey's refusal,
// which the server writes before it reads anything.
func dialPing(t *testing.T, addr string) (net.Conn, string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return c, ping(t, c)
}

func ping(t *testing.T, c net.Conn) string {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte("PING\r\n")); err != nil {
		return "write: " + err.Error()
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return "read: " + err.Error()
	}
	return line[:len(line)-2]
}

// The backup snapshot is a platform client: behind the internal proxy it dials
// the backend, taking the server's reserve and none of the tenant's budget.
func TestKeyValueBackupSnapshotBypassesTheTenantBudget(t *testing.T) {
	plan, _ := tiers.Valkey.ByID("starter")
	for _, c := range []struct {
		image        string
		internalAuth bool
		want         string
	}{
		{"bex@sha256:fixture", false, "red-kv-backend"},
		{"bex@sha256:fixture", true, "red-kv-backend"}, // the proxy fronts it too (w6/m463)
		{"", false, "red-kv"},                          // legacy embedding without a proxy
	} {
		r := &KeyValueReconciler{InternalProxyImage: c.image}
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-kv", Namespace: "tea-test"}, Spec: appv1alpha1.KeyValueSpec{Plan: "starter", InternalAuth: c.internalAuth}}
		spec := r.keyValueBackupCronJobSpec(kv, plan, "red-kv-platform")
		var host string
		for _, init := range spec.JobTemplate.Spec.Template.Spec.InitContainers {
			for _, e := range init.Env {
				if init.Name == "snapshot" && e.Name == "VALKEY_HOST" {
					host = e.Value
				}
			}
		}
		if host != c.want {
			t.Errorf("image %q internalAuth %v: snapshot dials %q, want %q", c.image, c.internalAuth, host, c.want)
		}
	}
}
