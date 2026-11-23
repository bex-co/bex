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
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// qualificationCNPGImage is the CloudNativePG operator production runs: bex's
// build of 1.30.1, whose instance manager keeps an extension whose library the
// cluster preloads (deploy/cnpg-operator, w21/m606). It must equal
// deploy/gitops/base/cnpg-operator.yaml's pin.
const qualificationCNPGImage = "ghcr.io/bex-co/bex-operator:cnpg-1.30.1-bex.1-07557237b85d3984fdf38665f8310288fb166596@sha256:5f8658442ad6979fb2c9060e3889dd5e54b13f4107d65243c67c79d9f88daa85"

// upstreamCNPGImage is the release that build patches, its runtime base. The
// matrix accepts it only to show what the patch changes: on it, the owner's
// pgaudit is gone after the restart.
const upstreamCNPGImage = "ghcr.io/cloudnative-pg/cloudnative-pg:1.30.1@sha256:923c267ec29636db3bee20f993d0ec4973fa22998e1adad37da79e4d32b5bc07"

func operandServerVersionNumber(t *testing.T, major string) string {
	t.Helper()
	data, err := os.ReadFile("../../../../deploy/pg-operand/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix map[string]struct{ Base string }
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatal(err)
	}
	ref, _, _ := strings.Cut(matrix[major].Base, "@")
	tag := ref[strings.LastIndex(ref, ":")+1:]
	version, _, _ := strings.Cut(tag, "-")
	parsedMajor, patch, ok := strings.Cut(version, ".")
	m, majorErr := strconv.Atoi(parsedMajor)
	p, patchErr := strconv.Atoi(patch)
	if !ok || parsedMajor != major || majorErr != nil || patchErr != nil {
		t.Fatalf("unrecognized exact base patch: %s", matrix[major].Base)
	}
	return strconv.Itoa(m*10000 + p)
}

func qualificationOperands(input, only string) (map[string]string, error) {
	images := map[string]string{}
	for pair := range strings.SplitSeq(input, ",") {
		major, image, ok := strings.Cut(pair, "=")
		_, supported := cnpgExportImages[major]
		if !ok || !supported || image == "" || images[major] != "" {
			return nil, fmt.Errorf("qualification requires unique supported <major>=<actual-image> pairs")
		}
		images[major] = image
	}
	majors := []string{"13", "14", "15", "16", "17", "18"}
	if only != "" {
		majors = strings.Split(only, ",")
	}
	selected := map[string]string{}
	for _, major := range majors {
		if images[major] == "" || selected[major] != "" {
			return nil, fmt.Errorf("missing or repeated PG%s qualification image", major)
		}
		selected[major] = images[major]
	}
	return selected, nil
}

func verifyCNPGOperandController(t *testing.T, cl client.Client, server string) {
	t.Helper()
	endpoint, err := url.Parse(server)
	if err != nil || endpoint.Scheme != "https" || (endpoint.Hostname() != "localhost" && !net.ParseIP(endpoint.Hostname()).IsLoopback()) {
		t.Fatal("operand qualification requires an isolated local API endpoint")
	}
	var deployments appsv1.DeploymentList
	if err := cl.List(context.Background(), &deployments, client.InNamespace("cnpg-system"), client.MatchingLabels{"app.kubernetes.io/name": "cloudnative-pg"}); err != nil {
		t.Fatal(err)
	}
	if len(deployments.Items) != 1 {
		t.Fatal("operand matrix requires exactly one pinned CNPG1.30.1 controller")
	}
	deployment := deployments.Items[0]
	images := make([]string, 0, len(deployment.Spec.Template.Spec.Containers))
	for _, container := range deployment.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}
	image := qualificationCNPGImage
	if !slices.Contains(images, image) {
		image = upstreamCNPGImage
	}
	if !slices.Contains(images, image) || !cnpgQualificationRolledOut(deployment) {
		t.Fatalf("CNPG1.30.1 must actually be Ready before matrix: images=%v ready=%d", images, deployment.Status.AvailableReplicas)
	}
	t.Logf("actual CNPG controller UID=%s image=%s available=%d", deployment.UID, image, deployment.Status.AvailableReplicas)
}

// TestQualificationCNPGImageIsProductionsPin: the matrix qualifies the
// operator production runs, which is the bex build and not the release it
// patches, and that release is the build's own base.
func TestQualificationCNPGImageIsProductionsPin(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/gitops/base/cnpg-operator.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var application struct {
		Spec struct {
			Source struct {
				Helm struct{ Values string } `json:"helm"`
			} `json:"source"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(raw, &application); err != nil {
		t.Fatal(err)
	}
	var values struct {
		Image struct{ Repository, Tag string } `json:"image"`
	}
	if err := yaml.Unmarshal([]byte(application.Spec.Source.Helm.Values), &values); err != nil {
		t.Fatal(err)
	}
	if pinned := values.Image.Repository + ":" + values.Image.Tag; pinned != qualificationCNPGImage {
		t.Errorf("production pins %q, the matrix qualifies %q", pinned, qualificationCNPGImage)
	}
	if strings.HasPrefix(qualificationCNPGImage, "ghcr.io/cloudnative-pg/") {
		t.Errorf("the matrix qualifies the upstream release %q, whose instance manager drops an owner's pgaudit (w21/m606)", qualificationCNPGImage)
	}
	dockerfile, err := os.ReadFile("../../../../deploy/cnpg-operator/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(dockerfile), "\n"), "FROM "+upstreamCNPGImage) {
		t.Errorf("deploy/cnpg-operator/Dockerfile is not built on %s", upstreamCNPGImage)
	}
}

func cnpgQualificationRolledOut(deployment appsv1.Deployment) bool {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	status := deployment.Status
	return desired > 0 && status.ObservedGeneration >= deployment.Generation &&
		status.Replicas == desired && status.UpdatedReplicas == desired &&
		status.ReadyReplicas == desired && status.AvailableReplicas == desired
}

func TestOperandQualificationRejectsOldAvailableControllerDuringRollout(t *testing.T) {
	deployment := appsv1.Deployment{}
	deployment.Generation = 2
	deployment.Status = appsv1.DeploymentStatus{ObservedGeneration: 2, Replicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	if cnpgQualificationRolledOut(deployment) {
		t.Fatal("old available replica passed before the pinned template rolled out")
	}
	deployment.Status.UpdatedReplicas = 1
	if !cnpgQualificationRolledOut(deployment) {
		t.Fatal("fully updated Ready controller was refused")
	}
	deployment.Status.ObservedGeneration = 1
	if cnpgQualificationRolledOut(deployment) {
		t.Fatal("stale status passed qualification")
	}
	deployment.Status.ObservedGeneration = 2
	deployment.Status.Replicas = 2
	if cnpgQualificationRolledOut(deployment) {
		t.Fatal("surplus old replica passed qualification")
	}
}

func TestOperandQualificationDoesNotSilentlyNarrowMatrix(t *testing.T) {
	for _, input := range []string{"18=actual", "13=one,13=other", "19=unknown", "13="} {
		if _, err := qualificationOperands(input, ""); err == nil {
			t.Errorf("incomplete/ambiguous matrix accepted: %s", input)
		}
	}
	if _, err := qualificationOperands("13=actual", "13"); err != nil {
		t.Fatal(err)
	}
}

// The original owner restore is a required outcome for every admitted
// extension. Its exact failure remains a red matrix result; a stronger
// platform actor never substitutes for it. TimescaleDB's pre/post-restore
// protocol runs only when it is admitted, so it is in the database.
func verifyCNPGOwnerExportRestore(t *testing.T, cl client.Client, kubeconfig, namespace string, timescale bool) {
	t.Helper()
	var secret corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: "pg-app"}, &secret); err != nil {
		t.Fatal(err)
	}
	ownerEnv := []string{"env", "PGHOST=localhost", "PGUSER=geo_app", "PGPASSWORD=" + string(secret.Data["password"])}
	run := func(arguments ...string) (string, error) {
		argv := []string{"--kubeconfig", kubeconfig, "-n", namespace, "exec", "pg-1", "-c", "postgres", "--"}
		body, err := exec.Command("kubectl", append(argv, arguments...)...).CombinedOutput()
		return string(body), err
	}
	if out, err := run("psql", "-X", "-v", "ON_ERROR_STOP=1", "-d", "postgres", "-c", "CREATE DATABASE owner_restored OWNER geo_app"); err != nil {
		t.Fatalf("prepare own restore database: %v %s", err, out)
	}
	if out, err := run(append(slices.Clone(ownerEnv), "PGDATABASE=geo_prod", "BEX_POSTGRES_EXPORT_ROOT=/tmp/bex-operand-owned-export", "EXPORT_DIRECTORY=fixture", "EXPORT_FILENAME=fixture.dir.tar.gz", "bex-postgres-export")...); err != nil {
		t.Fatalf("ordinary owner coherent export: %v %s", err, out)
	}
	restore := []string{"pg_restore", "--no-owner", "--no-privileges", "--exit-on-error", "--dbname=owner_restored", "/tmp/bex-operand-owned-export/fixture/geo_prod"}
	const sidecarFile = "/tmp/bex-operand-owned-export/fixture/pg_ivm_restore.sql"
	sidecar := []string{"psql", "-X", "-v", "ON_ERROR_STOP=1", "-d", "owner_restored", "-f", sidecarFile}
	check := "INSERT INTO ivm_base VALUES(4,40); SELECT (SELECT s FROM ivm_sum)=100 AND NOT rolsuper FROM pg_roles WHERE rolname=current_user"
	commands := make([][]string, 0, 4)
	if timescale {
		commands = append(commands,
			[]string{"psql", "-X", "-v", "ON_ERROR_STOP=1", "-d", "owner_restored", "-c", "CREATE EXTENSION timescaledb; SELECT timescaledb_pre_restore()"},
			restore,
			[]string{"psql", "-X", "-v", "ON_ERROR_STOP=1", "-d", "owner_restored", "-c", "SELECT timescaledb_post_restore()", "-f", sidecarFile})
		check = "INSERT INTO ivm_base VALUES(4,40); INSERT INTO metrics VALUES(now(),2); SELECT (SELECT s FROM ivm_sum)=100 AND (SELECT sum(v) FROM metrics)=3 AND NOT rolsuper FROM pg_roles WHERE rolname=current_user"
	} else {
		commands = append(commands, restore, sidecar)
	}
	commands = append(commands, []string{"psql", "-X", "-At", "-v", "ON_ERROR_STOP=1", "-d", "owner_restored", "-c", check})
	for i, command := range commands {
		out, err := run(append(slices.Clone(ownerEnv), command...)...)
		if err != nil || (i == len(commands)-1 && !strings.HasSuffix(strings.TrimSpace(out), "t")) {
			t.Errorf("ordinary-owner complete restore phase%d remains unqualified: %v %s", i, err, out)
			return
		}
	}
}
