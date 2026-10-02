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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func persistenceLiveFixture(t *testing.T) (*appv1alpha1.KeyValue, *appsv1.StatefulSet, *corev1.Pod, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, appv1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-persistence", Namespace: "test", UID: "kv-uid", Generation: 2}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "journal-snapshot"}}
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace, UID: "sts-uid", Generation: 1}, Spec: appsv1.StatefulSetSpec{Replicas: new(int32(1)), Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{keyValuePersistenceSourceAnnotation: "snapshot"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "valkey", Image: "valkey:8", Args: []string{"--appendonly", "no"}}}}}}, Status: appsv1.StatefulSetStatus{ObservedGeneration: 1, UpdateRevision: "snapshot-revision"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-0", Namespace: kv.Namespace, UID: "pod-uid", Labels: map[string]string{appsv1.StatefulSetRevisionLabel: "snapshot-revision"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "StatefulSet", Name: sts.Name, UID: sts.UID, Controller: new(true)}}}, Spec: *sts.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{PodIP: "127.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{Name: "valkey", ContainerID: "containerd://process-one", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(kv, pod, sts).WithObjects(kv, sts, pod).Build()
	return kv, sts, pod, c
}

func TestPersistenceSourcePodRejectsWrongRuntime(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*corev1.Pod, *appsv1.StatefulSet)
	}{
		{"old Off process", func(p *corev1.Pod, _ *appsv1.StatefulSet) {
			p.Spec.Containers[0].Args = []string{"--appendonly", "no", "--save", ""}
		}},
		{"old journal process", func(p *corev1.Pod, _ *appsv1.StatefulSet) {
			p.Spec.Containers[0].Args = []string{"--appendonly", "yes"}
		}},
		{"old image", func(p *corev1.Pod, _ *appsv1.StatefulSet) { p.Spec.Containers[0].Image = "valkey:7" }},
		{"old revision", func(p *corev1.Pod, _ *appsv1.StatefulSet) { p.Labels[appsv1.StatefulSetRevisionLabel] = "old" }},
		{"unobserved template", func(_ *corev1.Pod, s *appsv1.StatefulSet) { s.Generation++ }},
		{"missing revision", func(_ *corev1.Pod, s *appsv1.StatefulSet) { s.Status.UpdateRevision = "" }},
		{"unready", func(p *corev1.Pod, _ *appsv1.StatefulSet) { p.Status.Conditions[0].Status = corev1.ConditionFalse }},
		{"stopped process", func(p *corev1.Pod, _ *appsv1.StatefulSet) { p.Status.ContainerStatuses[0].State.Running = nil }},
		{"missing container ID", func(p *corev1.Pod, _ *appsv1.StatefulSet) { p.Status.ContainerStatuses[0].ContainerID = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, s, p, _ := persistenceLiveFixture(t)
			if !persistenceSourcePodReady(p, s) {
				t.Fatal("valid source rejected")
			}
			tc.mutate(p, s)
			if persistenceSourcePodReady(p, s) {
				t.Fatal("unsafe source accepted")
			}
		})
	}
}

func TestPersistenceProcessChangesOnSamePodRestart(t *testing.T) {
	_, _, p, _ := persistenceLiveFixture(t)
	initial := persistenceProcess(p)
	p.Status.ContainerStatuses[0].RestartCount++
	if persistenceProcess(p) == initial {
		t.Fatal("restart count must invalidate process identity")
	}
	p.Status.ContainerStatuses[0].RestartCount = 0
	p.Status.ContainerStatuses[0].ContainerID = "containerd://process-two"
	if persistenceProcess(p) == initial {
		t.Fatal("container replacement must invalidate process identity")
	}
}

type persistenceRedisHook struct{ process func(redis.Cmder) error }

func (h persistenceRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h persistenceRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h persistenceRedisHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, c redis.Cmder) error { return h.process(c) }
}

func TestPrepareValkeyJournalCompletionContract(t *testing.T) {
	complete := "aof_enabled:1\r\naof_rewrites:1\r\naof_rewrite_in_progress:0\r\naof_rewrite_scheduled:0\r\naof_last_bgrewrite_status:ok\r\naof_last_write_status:ok\r\n"
	for _, tc := range []struct {
		name, mode, info, fail    string
		ready, wantError, rewrite bool
	}{
		{name: "enable live journal", mode: "no", info: complete, ready: true},
		{name: "retry completed journal", mode: "yes", info: complete, ready: true},
		{name: "no rewrite completed", mode: "yes", info: strings.Replace(complete, "aof_rewrites:1", "aof_rewrites:0", 1)},
		{name: "rewrite running", mode: "yes", info: strings.Replace(complete, "aof_rewrite_in_progress:0", "aof_rewrite_in_progress:1", 1)},
		{name: "rewrite scheduled", mode: "yes", info: strings.Replace(complete, "aof_rewrite_scheduled:0", "aof_rewrite_scheduled:1", 1)},
		{name: "AOF disabled", mode: "no", info: strings.Replace(complete, "aof_enabled:1", "aof_enabled:0", 1)},
		{name: "incomplete INFO", mode: "yes", info: "aof_enabled:1\r\n"},
		{name: "rewrite failed", mode: "yes", info: strings.Replace(complete, "aof_last_bgrewrite_status:ok", "aof_last_bgrewrite_status:err", 1), wantError: true, rewrite: true},
		{name: "write failed", mode: "yes", info: strings.Replace(complete, "aof_last_write_status:ok", "aof_last_write_status:err", 1), wantError: true, rewrite: true},
		{name: "read settings error", mode: "no", fail: "config get", wantError: true},
		{name: "enable error", mode: "no", fail: "config set", wantError: true},
		{name: "INFO error", mode: "yes", fail: "info", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := redis.NewClient(&redis.Options{Addr: "unused:6379"})
			t.Cleanup(func() { _ = c.Close() })
			var commands []string
			c.AddHook(persistenceRedisHook{process: func(cmd redis.Cmder) error {
				args := cmd.Args()
				name := cmd.Name()
				if name == "config" {
					name += " " + fmt.Sprint(args[1])
				}
				commands = append(commands, name)
				if name == tc.fail {
					err := errors.New("injected Redis failure")
					cmd.SetErr(err)
					return err
				}
				switch name {
				case "config get":
					cmd.(*redis.MapStringStringCmd).SetVal(map[string]string{"appendonly": tc.mode})
				case "config set":
					if !reflect.DeepEqual(args, []any{"config", "set", "appendonly", "yes"}) {
						t.Fatalf("unexpected runtime mutation: %v", args)
					}
					cmd.(*redis.StatusCmd).SetVal("OK")
				case "info":
					cmd.(*redis.StringCmd).SetVal(tc.info)
				case "bgrewriteaof":
					cmd.(*redis.StatusCmd).SetVal("Background append only file rewriting started")
				default:
					t.Fatalf("unexpected command: %v", args)
				}
				return nil
			}})
			ready, err := prepareValkeyJournal(context.Background(), c)
			if ready != tc.ready || (err != nil) != tc.wantError {
				t.Fatalf("ready=%v error=%v, want ready=%v error=%v", ready, err, tc.ready, tc.wantError)
			}
			hasRewrite := false
			for _, cmd := range commands {
				hasRewrite = hasRewrite || cmd == "bgrewriteaof"
			}
			if hasRewrite != tc.rewrite {
				t.Fatalf("commands=%v, want retry rewrite=%v", commands, tc.rewrite)
			}
		})
	}
}

func TestPersistenceGateRetainsWorkloadWhileSourceUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*corev1.Pod)
	}{
		{"old Off process", func(p *corev1.Pod) { p.Spec.Containers[0].Args = []string{"--appendonly", "no", "--save", ""} }},
		{"unready process", func(p *corev1.Pod) { p.Status.Conditions[0].Status = corev1.ConditionFalse }},
		{"no process identity", func(p *corev1.Pod) { p.Status.ContainerStatuses = nil }},
		{"foreign owner", func(p *corev1.Pod) { p.OwnerReferences[0].UID = "different-sts" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv, sts, p, c := persistenceLiveFixture(t)
			tc.mutate(p)
			if err := c.Update(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			tc.mutate(p) // The fake main-resource update restores the stored status.
			if err := c.Status().Update(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			before := sts.DeepCopy()
			r := &KeyValueReconciler{Client: c}
			intent := keyValueIntent{}
			result, stop, err := r.prepareKeyValuePersistenceWith(context.Background(), kv, sts, &corev1.Secret{}, &intent, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
				t.Fatal("unsafe source reached live preparation")
				return false, nil
			})
			if err != nil || !stop || result.RequeueAfter == 0 {
				t.Fatalf("result=%+v stop=%v err=%v", result, stop, err)
			}
			current := &appsv1.StatefulSet{}
			if err := c.Get(context.Background(), client.ObjectKeyFromObject(sts), current); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before.Spec, current.Spec) || intent.persistenceToken != "" {
				t.Fatal("unsafe source authorized a rollout")
			}
			condition := meta.FindStatusCondition(kv.Status.Conditions, appv1alpha1.ConditionReady)
			if condition == nil || condition.Status != metav1.ConditionFalse || condition.ObservedGeneration != kv.Generation {
				t.Fatalf("readiness=%+v", condition)
			}
		})
	}
}

func TestPersistenceGateOnlyCommitsVerifiedLiveJournal(t *testing.T) {
	for _, tc := range []struct {
		name                string
		ready               bool
		prepareError        error
		restart, supersede  bool
		wantToken, wantStop bool
		phase               appv1alpha1.KeyValuePhase
	}{
		{name: "complete", ready: true, wantToken: true},
		{name: "rewrite pending", wantStop: true, phase: appv1alpha1.KVPhaseProvisioning},
		{name: "rewrite failed", prepareError: errors.New("password=must-not-leak"), wantStop: true, phase: appv1alpha1.KVPhaseFailed},
		{name: "same pod container restarted", ready: true, restart: true, wantStop: true},
		{name: "desired mode reversed", ready: true, supersede: true, wantToken: true, wantStop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			kv, sts, p, c := persistenceLiveFixture(t)
			before := sts.DeepCopy()
			r := &KeyValueReconciler{Client: c}
			intent := keyValueIntent{}
			auth := &corev1.Secret{Data: map[string][]byte{"password": []byte("exact-owned-credential")}}
			result, stop, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, auth, &intent, func(_ context.Context, actual *corev1.Pod, secret *corev1.Secret) (bool, error) {
				if actual.UID != p.UID || actual.Status.PodIP != p.Status.PodIP || secret != auth {
					t.Fatal("handoff target or credential changed")
				}
				if tc.restart {
					p.Status.ContainerStatuses[0].RestartCount++
					p.Status.ContainerStatuses[0].ContainerID = "containerd://restarted"
					if err := c.Status().Update(ctx, p); err != nil {
						t.Fatal(err)
					}
				}
				if tc.supersede {
					latest := kv.DeepCopy()
					latest.Generation++
					latest.Spec.PersistenceMode = "snapshot"
					if err := c.Update(ctx, latest); err != nil {
						t.Fatal(err)
					}
				}
				return tc.ready, tc.prepareError
			})
			if err != nil || stop != tc.wantStop {
				t.Fatalf("result=%+v stop=%v err=%v", result, stop, err)
			}
			stored := &appsv1.StatefulSet{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(sts), stored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before.Spec, stored.Spec) {
				t.Fatal("preparation changed serving pod template")
			}
			token := stored.Annotations[keyValuePersistenceTokenAnnotation]
			if (token != "") != tc.wantToken || intent.persistenceToken != token {
				t.Fatalf("persisted token=%q intent=%q wantToken=%v", token, intent.persistenceToken, tc.wantToken)
			}
			if tc.phase != "" {
				current := &appv1alpha1.KeyValue{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(kv), current); err != nil {
					t.Fatal(err)
				}
				condition := meta.FindStatusCondition(current.Status.Conditions, appv1alpha1.ConditionReady)
				if current.Status.Phase != tc.phase || condition == nil || condition.Status != metav1.ConditionFalse {
					t.Fatalf("status=%+v", current.Status)
				}
				if strings.Contains(condition.Message, "must-not-leak") {
					t.Fatal("network error exposed secret")
				}
			}
			if tc.supersede {
				latest := &appv1alpha1.KeyValue{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(kv), latest); err != nil {
					t.Fatal(err)
				}
				next := keyValueIntent{}
				_, blocked, err := r.prepareKeyValuePersistenceWith(ctx, latest, stored, auth, &next, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
					t.Fatal("completed superseded handoff was repeated")
					return false, nil
				})
				if err != nil || blocked || next.persistenceToken != token {
					t.Fatalf("reversal lost journal authority: blocked=%v err=%v token=%q", blocked, err, next.persistenceToken)
				}
			}
		})
	}
}

func TestPersistenceGateInvalidatesPreviousProcessPendingToken(t *testing.T) {
	for _, target := range []string{"snapshot", "journal-snapshot"} {
		t.Run(target, func(t *testing.T) {
			ctx := context.Background()
			kv, sts, p, c := persistenceLiveFixture(t)
			kv.Spec.PersistenceMode = target
			sts.Annotations = map[string]string{keyValuePersistenceTokenAnnotation: fmt.Sprintf("%s-%s-%d", p.UID, persistenceProcess(p), kv.Generation)}
			if err := c.Update(ctx, sts); err != nil {
				t.Fatal(err)
			}
			p.Status.ContainerStatuses[0].ContainerID = "containerd://new-process"
			// Readiness may lag the restart; identity alone invalidates the old proof.
			p.Status.Conditions[0].Status = corev1.ConditionFalse
			if err := c.Status().Update(ctx, p); err != nil {
				t.Fatal(err)
			}
			r := &KeyValueReconciler{Client: c}
			intent := keyValueIntent{}
			_, _, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, &corev1.Secret{}, &intent, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
				t.Fatal("unready new process must not be prepared")
				return false, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			stored := &appsv1.StatefulSet{}
			if err := c.Get(ctx, client.ObjectKeyFromObject(sts), stored); err != nil {
				t.Fatal(err)
			}
			if stored.Annotations[keyValuePersistenceTokenAnnotation] != "" || intent.persistenceToken != "" {
				t.Fatal("previous process token survived restart")
			}
		})
	}
}

func TestPersistenceGateWaitsForUnknownPendingTokenProcess(t *testing.T) {
	ctx := context.Background()
	kv, sts, p, c := persistenceLiveFixture(t)
	kv.Spec.PersistenceMode = "snapshot"
	sts.Annotations = map[string]string{keyValuePersistenceTokenAnnotation: fmt.Sprintf("%s-%s-%d", p.UID, persistenceProcess(p), kv.Generation)}
	if err := c.Update(ctx, sts); err != nil {
		t.Fatal(err)
	}
	p.Status.ContainerStatuses[0].State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}
	p.Status.Conditions[0].Status = corev1.ConditionFalse
	if err := c.Status().Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	r := &KeyValueReconciler{Client: c}
	intent := keyValueIntent{}
	result, stop, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, &corev1.Secret{}, &intent, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
		t.Fatal("unknown process prepared")
		return false, nil
	})
	if err != nil || !stop || result.RequeueAfter == 0 {
		t.Fatalf("unknown process authorized stale journal: result=%+v stop=%v err=%v", result, stop, err)
	}
}

func TestPersistenceGateFencesAgainstFreshPodRead(t *testing.T) {
	ctx := context.Background()
	kv, sts, p, cached := persistenceLiveFixture(t)
	actual := fake.NewClientBuilder().WithScheme(cached.Scheme()).WithStatusSubresource(p, kv, sts).WithObjects(p, kv, sts).Build()
	r := &KeyValueReconciler{Client: cached, APIReader: actual}
	intent := keyValueIntent{}
	_, stop, err := r.prepareKeyValuePersistenceWith(ctx, kv, sts, &corev1.Secret{}, &intent, func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error) {
		p.Status.ContainerStatuses[0].ContainerID = "containerd://replacement"
		if err := actual.Status().Update(ctx, p); err != nil {
			t.Fatal(err)
		}
		return true, nil
	})
	if err != nil || !stop || intent.persistenceToken != "" {
		t.Fatalf("stale informer pod authorized rollout: stop=%v err=%v token=%q", stop, err, intent.persistenceToken)
	}
}
