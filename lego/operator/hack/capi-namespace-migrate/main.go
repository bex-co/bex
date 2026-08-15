// capi-namespace-migrate performs a same-management-cluster Cluster API move
// while rewriting the namespace of the moved object graph. It exists only for
// the one-time production migration from default to bex-capi.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	clusterctlclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/cluster"
	logf "sigs.k8s.io/cluster-api/cmd/clusterctl/log"
)

var namespacedReferencePaths = map[string][][]string{
	"Cluster": {
		{"spec", "controlPlaneRef", "namespace"},
		{"spec", "infrastructureRef", "namespace"},
	},
	"KubeadmControlPlane": {
		{"spec", "machineTemplate", "infrastructureRef", "namespace"},
	},
	"Machine": {
		{"spec", "bootstrap", "configRef", "namespace"},
		{"spec", "infrastructureRef", "namespace"},
	},
	"MachineDeployment": {
		{"spec", "template", "spec", "bootstrap", "configRef", "namespace"},
		{"spec", "template", "spec", "infrastructureRef", "namespace"},
	},
	"MachineSet": {
		{"spec", "template", "spec", "bootstrap", "configRef", "namespace"},
		{"spec", "template", "spec", "infrastructureRef", "namespace"},
	},
	"MachinePool": {
		{"spec", "template", "spec", "bootstrap", "configRef", "namespace"},
		{"spec", "template", "spec", "infrastructureRef", "namespace"},
	},
}

func namespaceMutator(fromNamespace, toNamespace string) cluster.ResourceMutatorFunc {
	return func(obj *unstructured.Unstructured) error {
		if obj == nil || obj.Object == nil {
			return nil
		}
		if obj.GetNamespace() == fromNamespace {
			obj.SetNamespace(toNamespace)
		}
		for _, path := range namespacedReferencePaths[obj.GetKind()] {
			value, found, err := unstructured.NestedString(obj.Object, path...)
			if err != nil {
				return fmt.Errorf("read %s namespace reference %v: %w", obj.GetKind(), path, err)
			}
			if !found || value != fromNamespace {
				continue
			}
			if err := unstructured.SetNestedField(obj.Object, toNamespace, path...); err != nil {
				return fmt.Errorf("rewrite %s namespace reference %v: %w", obj.GetKind(), path, err)
			}
		}
		return nil
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("capi-namespace-migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	kubeconfig := flags.String("kubeconfig", "", "management-cluster kubeconfig path")
	fromNamespace := flags.String("from-namespace", "", "source namespace")
	toNamespace := flags.String("to-namespace", "", "target namespace")
	confirmation := flags.String("confirm", "", "required destructive-operation confirmation")
	dryRun := flags.Bool("dry-run", false, "run clusterctl discovery and move preflight without writes")
	backupDirectory := flags.String("backup-directory", "", "back up the source graph without deleting it")
	restoreDirectory := flags.String("restore-directory", "", "restore a previously backed-up source graph")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *kubeconfig == "" || *fromNamespace == "" || *toNamespace == "" {
		return errors.New("kubeconfig, from-namespace, and to-namespace are required")
	}
	if *fromNamespace == *toNamespace {
		return errors.New("source and target namespaces must differ")
	}
	modeCount := 0
	for _, enabled := range []bool{*dryRun, *backupDirectory != "", *restoreDirectory != ""} {
		if enabled {
			modeCount++
		}
	}
	if modeCount > 1 {
		return errors.New("dry-run, backup-directory, and restore-directory are mutually exclusive")
	}
	expectedConfirmation := fmt.Sprintf("MOVE-%s-TO-%s", *fromNamespace, *toNamespace)
	if *restoreDirectory != "" {
		expectedConfirmation = fmt.Sprintf("RESTORE-%s", *fromNamespace)
	}
	if !*dryRun && *backupDirectory == "" && *confirmation != expectedConfirmation {
		return fmt.Errorf("confirmation must be %q", expectedConfirmation)
	}

	client, err := clusterctlclient.New(ctx, "")
	if err != nil {
		return fmt.Errorf("create clusterctl client: %w", err)
	}
	config := clusterctlclient.Kubeconfig{Path: *kubeconfig}
	options := clusterctlclient.MoveOptions{
		FromKubeconfig: config,
		ToKubeconfig:   config,
		Namespace:      *fromNamespace,
		DryRun:         *dryRun,
		ExperimentalResourceMutators: []cluster.ResourceMutatorFunc{
			namespaceMutator(*fromNamespace, *toNamespace),
		},
	}
	if *backupDirectory != "" {
		options.ToKubeconfig = clusterctlclient.Kubeconfig{}
		options.ToDirectory = *backupDirectory
		options.ExperimentalResourceMutators = nil
	}
	if *restoreDirectory != "" {
		options.FromKubeconfig = clusterctlclient.Kubeconfig{}
		options.FromDirectory = *restoreDirectory
		options.ExperimentalResourceMutators = nil
	}
	if err := client.Move(ctx, options); err != nil {
		return fmt.Errorf("move CAPI object graph: %w", err)
	}
	return nil
}

func main() {
	logf.SetLogger(klog.Background())
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "capi-namespace-migrate: %v\n", err)
		os.Exit(1)
	}
}
