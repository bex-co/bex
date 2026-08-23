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
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func diskApp(name string, disk *appv1alpha1.DiskSpec) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Name = name
	app.Namespace = "default"
	app.Spec.Image = "nginx:1"
	app.Spec.Disk = disk
	return app
}

func projectDisk(app *appv1alpha1.App, dep *appsv1.Deployment) *corev1.Container {
	container := &corev1.Container{Name: "app"}
	applyDiskProjection(dep, container, app)
	return container
}

func TestApplyDiskProjectionMountsTheVolumeAndForcesRecreate(t *testing.T) {
	app := diskApp("blog", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10})
	dep := &appsv1.Deployment{}

	container := projectDisk(app, dep)

	if dep.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType {
		t.Fatalf("strategy = %q, want Recreate: two revisions must never write one volume", dep.Spec.Strategy.Type)
	}
	if got := dep.Spec.Template.Annotations[diskSafeToEvictAnnotation]; got != "false" {
		t.Fatalf("safe-to-evict = %q, want \"false\" so the autoscaler cannot consolidate the node away", got)
	}
	if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].MountPath != "/var/data" {
		t.Fatalf("volume mounts = %+v, want one mount at /var/data", container.VolumeMounts)
	}
	if len(dep.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("volumes = %+v, want exactly the disk volume", dep.Spec.Template.Spec.Volumes)
	}
	claim := dep.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim
	if claim == nil || claim.ClaimName != "disk-blog" {
		t.Fatalf("volume source = %+v, want the disk-blog claim", dep.Spec.Template.Spec.Volumes[0].VolumeSource)
	}
}

// A stateless App must reconcile byte-identically to a pre-disk operator, or
// this feature silently rolls the entire fleet.
func TestApplyDiskProjectionLeavesADisklessDeploymentUntouched(t *testing.T) {
	app := diskApp("stateless", nil)
	dep := &appsv1.Deployment{}
	dep.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}

	container := projectDisk(app, dep)

	if dep.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType {
		t.Fatalf("strategy = %q, want the stored RollingUpdate left alone", dep.Spec.Strategy.Type)
	}
	if len(container.VolumeMounts) != 0 || len(dep.Spec.Template.Spec.Volumes) != 0 {
		t.Fatalf("disk-less App gained volumes: mounts=%+v volumes=%+v", container.VolumeMounts, dep.Spec.Template.Spec.Volumes)
	}
}

func TestApplyDiskProjectionRestoresRollingDeploysWhenTheDiskIsDetached(t *testing.T) {
	attached := diskApp("blog", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10})
	dep := &appsv1.Deployment{}
	projectDisk(attached, dep)

	// Same Deployment object, disk now gone from the spec — the shape the
	// reconciler sees on the pass right after a detach.
	detached := diskApp("blog", nil)
	container := projectDisk(detached, dep)

	if dep.Spec.Strategy.Type != "" {
		t.Fatalf("strategy = %q, want cleared so the API server defaults it back to RollingUpdate", dep.Spec.Strategy.Type)
	}
	if _, ok := dep.Spec.Template.Annotations[diskSafeToEvictAnnotation]; ok {
		t.Fatal("safe-to-evict survived detach; the pod would stay pinned against consolidation forever")
	}
	if len(container.VolumeMounts) != 0 {
		t.Fatalf("volume mounts = %+v, want none after detach", container.VolumeMounts)
	}
}

// The projection helper is only correct if the real Deployment builder actually
// calls it, and calls it after the container exists — so assert through
// applyDeploymentSpec, the entry point the reconciler uses.
func TestDeploymentSpecCarriesTheDiskAlongsideSecretFiles(t *testing.T) {
	app := diskApp("blog", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10})
	app.Spec.FilesFromSecrets = []string{"blog-files"}
	dep := &appsv1.Deployment{}

	applyDeploymentSpec(dep, app, deploymentParams{image: "nginx:1", port: 3000, replicas: 1})

	if dep.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType {
		t.Fatalf("strategy = %q, want Recreate", dep.Spec.Strategy.Type)
	}
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(dep.Spec.Template.Spec.Containers))
	}
	// The secret-files mount must survive beside the disk: the disk projection
	// appends, and an earlier version of this code overwrote instead.
	containerMounts := dep.Spec.Template.Spec.Containers[0].VolumeMounts
	mounts := make([]string, 0, len(containerMounts))
	for _, m := range containerMounts {
		mounts = append(mounts, m.MountPath)
	}
	if len(mounts) != 2 {
		t.Fatalf("volume mounts = %v, want both the secrets mount and the disk", mounts)
	}
	if len(dep.Spec.Template.Spec.Volumes) != 2 {
		t.Fatalf("volumes = %d, want the projected secrets volume and the disk claim", len(dep.Spec.Template.Spec.Volumes))
	}
}

func TestDiskChildNamesStayWithinKubernetesLimits(t *testing.T) {
	long := strings.Repeat("a", 63)
	for _, name := range []string{"blog", long} {
		pvc, secret := diskPVCName(name), diskLUKSSecretName(name)
		if len(pvc) > 63 || len(secret) > 63 {
			t.Fatalf("names too long for %q: pvc=%d secret=%d", name, len(pvc), len(secret))
		}
		if pvc == secret {
			t.Fatalf("claim and passphrase collided on %q: %q", name, pvc)
		}
	}
	// Two long names sharing a prefix must not collide onto one volume.
	a := diskPVCName(strings.Repeat("a", 60) + "one")
	b := diskPVCName(strings.Repeat("a", 60) + "two")
	if a == b {
		t.Fatalf("distinct long App names collided onto one claim: %q", a)
	}
}

// Release identity is what decides whether a change rolls the pod. Growing a
// disk is applied online, so it must not; remounting or attaching one rewrites
// the pod template, so it must.
func TestDiskGrowIsNotARelease(t *testing.T) {
	base := appv1alpha1.AppSpec{
		Image: "nginx:1", Port: 3000,
		Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10},
	}
	grown := base
	grown.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 50}

	if desiredAppReleaseIdentity(base).release != desiredAppReleaseIdentity(grown).release {
		t.Fatal("growing a disk changed the release identity; an online resize would redeploy the service")
	}
}

func TestDiskAttachAndRemountAreReleases(t *testing.T) {
	stateless := appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000}
	attached := stateless
	attached.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	remounted := stateless
	remounted.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/srv/data", SizeGB: 10}

	statelessID := desiredAppReleaseIdentity(stateless).release
	attachedID := desiredAppReleaseIdentity(attached).release
	remountedID := desiredAppReleaseIdentity(remounted).release

	if statelessID == attachedID {
		t.Fatal("attaching a disk did not change the release identity; the pod would never gain the volume")
	}
	if attachedID == remountedID {
		t.Fatal("changing mountPath did not change the release identity; the pod would keep the old mount")
	}
}
