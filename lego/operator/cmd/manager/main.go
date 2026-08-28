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

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"os"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/bex-co/bex/lego/operator/internal/controller"
	"github.com/bex-co/bex/lego/operator/internal/hostingdomain"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	bexruntime "github.com/bex-co/bex/lego/operator/internal/runtime"
	"github.com/bex-co/bex/lego/operator/internal/selfimage"
	bexwebhook "github.com/bex-co/bex/lego/operator/internal/webhook"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const defaultCNBBuilder = "paketobuildpacks/builder-jammy-base@" +
	"sha256:5799343cd316c1a03fa3ff7ab0915d9e6d134e95df4583016d70c6f5330d3898"

// envOr returns the env var k or a default.
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// runtimeMode resolves the workload runtime. Kubernetes is the default because
// it is the only runtime production deploys and the only one under test: an
// unset BEX_RUNTIME used to select the opensandbox host path, which also
// early-returns out of registry-credential convergence, so a forgotten variable
// degraded the operator silently rather than loudly.
func runtimeMode() string {
	return envOr("BEX_RUNTIME", controller.ModeKubernetes)
}

// positiveEnvInt returns a strictly positive integer from k, or def when k is
// unset, malformed, zero, or negative. Controller worker counts must never be
// zero: controller-runtime interprets an unset value as its default of one.
func positiveEnvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}

// envInt returns the integer from k, or def when k is unset or malformed. Any
// parsed value — including zero and negatives — is accepted as-is.
func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// managerConfig carries the flag- and env-derived settings main needs before
// the manager is constructed.
type managerConfig struct {
	metricsAddr          string
	metricsCertPath      string
	metricsCertName      string
	metricsCertKey       string
	webhookCertPath      string
	webhookCertName      string
	webhookCertKey       string
	enableLeaderElection bool
	probeAddr            string
	secureMetrics        bool
	enableHTTP2          bool
	baseDomain           string
}

// parseManagerConfig registers and parses the manager flags, installs the
// logger, and validates the shared tenant hosting suffix.
func parseManagerConfig() managerConfig {
	var cfg managerConfig
	flag.StringVar(&cfg.metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&cfg.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&cfg.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&cfg.secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&cfg.webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&cfg.webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&cfg.webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&cfg.metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&cfg.metricsCertName, "metrics-cert-name", "tls.crt",
		"The name of the metrics server certificate file.")
	flag.StringVar(&cfg.metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&cfg.enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	cfg.baseDomain = os.Getenv("BEX_BASE_DOMAIN")
	if err := hostingdomain.ValidateSharedSuffix(cfg.baseDomain); err != nil {
		// A shared host suffix is a browser security boundary; an unlisted suffix
		// lets one tenant set cookies its siblings receive. But per the standing
		// #PSL decision (.pm/DO_NOT_DO.md — the finding is ACCEPTED until open
		// signup; onbex.co cannot be listed yet, and two production outages came
		// from "remediating" this by disabling the domain), an unlisted-but-well-
		// formed suffix WARNS LOUDLY AND KEEPS SERVING. Only a malformed domain
		// refuses startup.
		if errors.Is(err, hostingdomain.ErrUnlistedSharedSuffix) {
			setupLog.Error(err,
				"shared tenant hosting suffix is not a private Public Suffix; continuing per the accepted #PSL risk "+
					"(.pm/DO_NOT_DO.md) — cross-tenant cookie isolation is NOT browser-enforced")
		} else {
			setupLog.Error(err, "unsafe shared tenant hosting suffix; refusing startup")
			os.Exit(1)
		}
	}
	return cfg
}

// buildServerOptions assembles the webhook server and the metrics-server
// options from the parsed manager config.
func buildServerOptions(cfg managerConfig) (webhook.Server, metricsserver.Options) {
	var tlsOpts []func(*tls.Config)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !cfg.enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(cfg.webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", cfg.webhookCertPath, "webhook-cert-name", cfg.webhookCertName,
			"webhook-cert-key", cfg.webhookCertKey)

		webhookServerOptions.CertDir = cfg.webhookCertPath
		webhookServerOptions.CertName = cfg.webhookCertName
		webhookServerOptions.KeyName = cfg.webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	metricsServerOptions := metricsserver.Options{
		BindAddress:   cfg.metricsAddr,
		SecureServing: cfg.secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if cfg.secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// Without a certificate, controller-runtime auto-generates self-signed
	// metrics-server certificates — not recommended for production.
	if len(cfg.metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", cfg.metricsCertPath, "metrics-cert-name", cfg.metricsCertName,
			"metrics-cert-key", cfg.metricsCertKey)

		metricsServerOptions.CertDir = cfg.metricsCertPath
		metricsServerOptions.CertName = cfg.metricsCertName
		metricsServerOptions.KeyName = cfg.metricsCertKey
	}

	return webhookServer, metricsServerOptions
}

func main() {
	cfg := parseManagerConfig()

	webhookServer, metricsServerOptions := buildServerOptions(cfg)

	appsNamespace := envOr("BEX_APPS_NAMESPACE", "default")
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Cache:                  controller.NamespacedSecretCacheOptions(appsNamespace),
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.probeAddr,
		LeaderElection:         cfg.enableLeaderElection,
		LeaderElectionID:       "36450c48.bex.co",
		// LeaderElectionReleaseOnCancel: true, // voluntary step-down on stop;
		// only safe when the binary exits immediately after the manager stops.
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	// Build a kubernetes clientset for the metrics-server reader (autoscaling).
	// A failure here is not fatal — the autoscaler is skipped when MetricsReader
	// is nil, so existing behavior is preserved.
	cs, csErr := kubernetes.NewForConfig(mgr.GetConfig())
	if csErr != nil {
		setupLog.Info("metrics-server reader unavailable; autoscaling disabled", "reason", csErr)
	}
	// Secrets outside BEX_APPS_NAMESPACE are deliberately absent from the
	// manager cache. Keep one direct client for build-plane resources and other
	// explicitly RBAC-scoped namespaces such as the Zot registry namespace.
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		setupLog.Error(err, "Failed to create uncached client")
		os.Exit(1)
	}

	setupAppReconciler(mgr, uncachedClient, cs, appsNamespace, cfg.baseDomain)
	controller.RegisterClusterBuilderMetrics(uncachedClient)
	// Datastore readiness (w7/036). A Database that never provisions is otherwise
	// invisible: CNPG's metrics come from instance pods, so a Cluster that never
	// got one exports nothing, and "nothing" is precisely the state worth paging
	// on. Uncached, like the ClusterBuilder collector, so a scrape never depends
	// on informer state.
	controller.RegisterDatastoreMetrics(uncachedClient)

	setupDatastoreReconcilers(mgr, uncachedClient, appsNamespace)
	// +kubebuilder:scaffold:builder

	// Admission-time tenant-image signature verification (w7/m11): active only
	// when BEX_TENANT_SIGNING_KEY_SECRET is set AND the Secret contains cosign.pub.
	if signKey := os.Getenv("BEX_TENANT_SIGNING_KEY_SECRET"); signKey != "" {
		if err := bexwebhook.SetupWithManager(mgr,
			signKey,
			envOr("BEX_BUILD_NAMESPACE", "bex-system"),
			envOr("BEX_REGISTRY", "127.0.0.1:5050"),
			os.Getenv("BEX_REGISTRY_PUSH_SECRET"),
		); err != nil {
			setupLog.Error(err, "Failed to set up image signature admission webhook")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}

// setupAppReconciler constructs and registers the App reconciler (and, when
// per-App registry credentials are active, the sandbox-namespace registry
// reconciler).
func setupAppReconciler(
	mgr ctrl.Manager, uncachedClient client.Client, cs *kubernetes.Clientset, appsNamespace, baseDomain string,
) {
	activatorPort := envInt("BEX_ACTIVATOR_PORT", 8888)
	appReconciler := &controller.AppReconciler{
		Client:               mgr.GetClient(),
		BuildClient:          uncachedClient,
		Scheme:               mgr.GetScheme(),
		AppsNamespace:        appsNamespace,
		Mode:                 runtimeMode(),
		Registry:             envOr("BEX_REGISTRY", "127.0.0.1:5050"),
		KpackRegistry:        os.Getenv("BEX_KPACK_REGISTRY"),
		CNBBuilder:           envOr("BEX_CNB_BUILDER", defaultCNBBuilder),
		BuildNamespace:       os.Getenv("BEX_BUILD_NAMESPACE"),
		Runtime:              bexruntime.New(envOr("BEX_OPENSANDBOX_URL", "http://127.0.0.1:8077")),
		BaseDomain:           baseDomain,
		ClusterIssuer:        envOr("BEX_CLUSTER_ISSUER", "letsencrypt-staging"),
		ActivatorService:     envOr("BEX_ACTIVATOR_SERVICE", ""),
		ActivatorNamespace:   envOr("POD_NAMESPACE", "bex-system"),
		ActivatorPort:        activatorPort,
		MaintenanceService:   envOr("BEX_ACTIVATOR_SERVICE", "bex-activator"),
		MaintenanceNamespace: envOr("POD_NAMESPACE", "bex-system"),
		MaintenancePort:      activatorPort,
		DiskStorageClass:     os.Getenv("BEX_DISK_STORAGE_CLASS"),
		BackupHelperImage:    backupHelperImage(uncachedClient),
		// Nightly persistent-disk snapshots (ADR082 D5). Every field must be
		// present or no snapshot is taken: a disk snapshot is a full copy of a
		// tenant filesystem leaving the cluster, so a half-configured store
		// would either write nowhere or write it unencrypted.
		DiskSnapshots: controller.DiskSnapshotStore{
			Endpoint:     os.Getenv("BEX_DISK_SNAPSHOT_ENDPOINT"),
			Bucket:       os.Getenv("BEX_DISK_SNAPSHOT_BUCKET"),
			Prefix:       os.Getenv("BEX_DISK_SNAPSHOT_PREFIX"),
			Region:       os.Getenv("BEX_DISK_SNAPSHOT_REGION"),
			S3Secret:     os.Getenv("BEX_DISK_SNAPSHOT_S3_SECRET"),
			AgePublicKey: os.Getenv("BEX_DISK_SNAPSHOT_AGE_PUBLIC_KEY"),
			AgeSecret:    os.Getenv("BEX_DISK_SNAPSHOT_AGE_SECRET"),
		},
		MaxConcurrentBuilds:     envInt("BEX_MAX_CONCURRENT_BUILDS", 0),
		MaxActiveBuilds:         envInt("BEX_MAX_ACTIVE_BUILDS", 0),
		MaxConcurrentReconciles: positiveEnvInt("BEX_APP_RECONCILE_WORKERS", 1),
		StaticStore: publish.Store{
			Bucket:   envOr("BEX_STATIC_S3_BUCKET", ""),
			Endpoint: envOr("BEX_STATIC_S3_ENDPOINT", ""),
			Region:   envOr("BEX_STATIC_S3_REGION", ""),
			Secret:   envOr("BEX_STATIC_PUBLISH_S3_SECRET", ""),
		},
		StaticServerService:     envOr("BEX_STATIC_SERVER_SERVICE", ""),
		StaticServerNamespace:   envOr("POD_NAMESPACE", "bex-system"),
		StaticServerPort:        envInt("BEX_STATIC_SERVER_PORT", 8080),
		TenantSignKeySecret:     os.Getenv("BEX_TENANT_SIGNING_KEY_SECRET"),
		TenantSignImage:         envOr("BEX_TENANT_SIGNING_IMAGE", ""),
		RegistryPushSecret:      os.Getenv("BEX_REGISTRY_PUSH_SECRET"),
		RegistryPullSecret:      os.Getenv("BEX_REGISTRY_PULL_SECRET"),
		RegistryBuildPullSecret: os.Getenv("BEX_REGISTRY_BUILD_PULL_SECRET"),
		// Per-App registry layer cache (docs/ADR060 D3). The variable takes a
		// backend NAME rather than a truthy flag so that the escalation ADR060
		// already names — per-workspace persistent cache volumes — arrives as
		// another value here instead of a second variable. Anything else
		// (including unset) leaves the build Job byte-identical to before the
		// feature existed.
		BuildCache: os.Getenv("BEX_BUILD_CACHE") == "registry",
	}
	// Build-namespace pull credential for build-plane Jobs (the static-site publish
	// Job) that pull the just-built tenant image from Zot. The per-App/shared tenant
	// pull secret lives in the App namespace and is unreachable when the build Job
	// runs in a separate BEX_BUILD_NAMESPACE, so this is a distinct build-ns secret
	// (scripts/registry-secrets.sh's bex-registry-pull, bex-builder with wildcard
	// read). Default to that conventional name whenever the push credential is set
	// (i.e. Zot auth is enabled); unset in dev ⇒ anonymous pull, byte-identical.
	if appReconciler.RegistryBuildPullSecret == "" && appReconciler.RegistryPushSecret != "" {
		appReconciler.RegistryBuildPullSecret = "bex-registry-pull"
	}
	// Per-App registry pull credentials (w7/m36). Active when BEX_REGISTRY_NS is
	// set (typically "bex-registry"). Supersedes the shared bex-puller path.
	if zotNS := os.Getenv("BEX_REGISTRY_NS"); zotNS != "" {
		appReconciler.PerAppRegistry = &registry.Creds{
			// Zot Secrets live outside the manager's namespaced Secret cache.
			// A cached client fails these reads with "unknown namespace for the
			// cache" before any build can start.
			Client:         uncachedClient,
			ZotNamespace:   zotNS,
			HTPasswdName:   envOr("BEX_ZOT_HTPASSWD_SECRET", "zot-htpasswd"),
			ConfigName:     envOr("BEX_ZOT_CONFIG_SECRET", "zot-config"),
			Registry:       envOr("BEX_REGISTRY", "127.0.0.1:5050"),
			KpackRegistry:  os.Getenv("BEX_KPACK_REGISTRY"),
			RetentionCount: positiveEnvInt("BEX_ZOT_RETENTION_COUNT", 0),
			// Legacy-repository dual-read is off unless an operator opts in for a
			// supervised registry migration (round-21 finding 4): the compatibility
			// grant is keyed by the bare App name with no ownership check, so leaving
			// it always-on lets a scoped App read a same-named legacy repo owned by
			// another workspace.
			DualReadEnabled: os.Getenv("BEX_REGISTRY_DUAL_READ") == "1",
		}
	}
	if cs != nil {
		appReconciler.MetricsReader = controller.NewMetricsServerReader(cs)
	}
	if err := appReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "service")
		os.Exit(1)
	}

	// Per-workspace sandbox snapshot resume-pull credentials (w3/m42 t002,
	// ADR042 D5): active alongside the per-App registry credentials whenever
	// BEX_REGISTRY_NS enables Zot credential management. Each namespace the
	// control plane labels app.bex.co/regime=sandbox gets a read-only Zot user
	// scoped to its own "snapshots/<ns>/**" repositories plus the in-namespace
	// bex-snapshot-pull Secret the OpenSandbox controller's
	// --resume-pull-secret flag references.
	if appReconciler.PerAppRegistry != nil {
		if err := (&controller.SandboxNamespaceRegistryReconciler{
			Client:   uncachedClient,
			Scheme:   mgr.GetScheme(),
			Registry: appReconciler.PerAppRegistry,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Failed to create controller", "controller", "sandboxnamespaceregistry")
			os.Exit(1)
		}
	}
}

// setupDatastoreReconcilers constructs and registers the Database and KeyValue
// reconcilers.
// backupHelperImage resolves the image the Key Value backup CronJob's encrypt
// stage runs — the operator's own, so that stage executes a first-party
// entrypoint (/backup-encrypt) from an artifact bex builds, signs and pins
// rather than fetching age at run time (w7/m85).
//
// BEX_BACKUP_HELPER_IMAGE overrides it for deployments that run the manager
// outside a Pod. Empty is not fatal here: backups without encryption are
// unaffected, and the encryption path fails closed in the reconciler with a
// message naming both remedies.
func backupHelperImage(c client.Client) string {
	if explicit := envOr("BEX_BACKUP_HELPER_IMAGE", ""); explicit != "" {
		return explicit
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	image, err := selfimage.Resolve(ctx, c,
		envOr("POD_NAMESPACE", ""), envOr("POD_NAME", ""), selfimage.ManagerContainer)
	if err != nil {
		setupLog.Info("backup helper image unresolved; KeyValue backup ENCRYPTION will fail closed "+
			"(unencrypted backups are unaffected)", "reason", err.Error())
		return ""
	}
	return image
}

func setupDatastoreReconcilers(mgr ctrl.Manager, uncachedClient client.Client, appsNamespace string) {
	databaseReconciler := &controller.DatabaseReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		// Datastore Secrets live beside their CR in the workspace's own
		// namespace (ADR043 D8), which the manager's single-namespace Secret
		// informer does not cover — see the field's doc.
		SecretClient: uncachedClient,
		// The GitOps-installed ObjectStore + S3 credential live here; the
		// operator projects them into each tenant namespace (ADR043 D8.4).
		BackupSourceNamespace: appsNamespace,
		// DatabaseReconciler still consumes client-go's record.EventRecorder;
		// controller-runtime's replacement uses the incompatible events API.
		Recorder: mgr.GetEventRecorderFor("database-controller"), //nolint:staticcheck
		DBDomain: envOr("BEX_DB_DOMAIN", ""),
		Backup: controller.BackupStore{
			DestinationPath: envOr("BEX_DB_BACKUP_DESTINATION", ""),
			EndpointURL:     envOr("BEX_DB_BACKUP_ENDPOINT", ""),
			S3Secret:        envOr("BEX_DB_BACKUP_S3_SECRET", ""),
		},
	}
	if promURL := os.Getenv("BEX_PROM_URL"); promURL != "" {
		databaseReconciler.DiskUsageReader = controller.NewPrometheusDatabaseDiskUsageReader(promURL, nil)
	}
	if err := databaseReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "database")
		os.Exit(1)
	}

	kvBackupDestination := envOr("BEX_KV_BACKUP_DESTINATION", "")
	kvBackupEndpoint := envOr("BEX_KV_BACKUP_ENDPOINT", "")
	kvBackupSecret := envOr("BEX_KV_BACKUP_S3_SECRET", "")
	configuredKVBackupFields := 0
	for _, value := range []string{kvBackupDestination, kvBackupEndpoint, kvBackupSecret} {
		if value != "" {
			configuredKVBackupFields++
		}
	}
	if configuredKVBackupFields > 0 && configuredKVBackupFields < 3 {
		setupLog.Info("KeyValue backups disabled: BEX_KV_BACKUP_* contract is incomplete",
			"destinationConfigured", kvBackupDestination != "",
			"endpointConfigured", kvBackupEndpoint != "",
			"secretNameConfigured", kvBackupSecret != "")
	}
	if err := (&controller.KeyValueReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		SecretClient:          uncachedClient, // see DatabaseReconciler above
		BackupSourceNamespace: appsNamespace,
		BackupHelperImage:     backupHelperImage(uncachedClient),
		KvDomain:              envOr("BEX_KV_DOMAIN", ""),
		ClusterIssuer:         envOr("BEX_CLUSTER_ISSUER", ""),
		Backup: controller.BackupStore{
			DestinationPath: kvBackupDestination,
			EndpointURL:     kvBackupEndpoint,
			S3Secret:        kvBackupSecret,
			// ADR050 Tier A opt-in: recipient public key for client-side age
			// encryption of KeyValue RDB snapshots. Empty ⇒ plain upload.
			AgePublicKey: envOr("BEX_BACKUP_AGE_PUBLIC_KEY", ""),
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "keyvalue")
		os.Exit(1)
	}
}
