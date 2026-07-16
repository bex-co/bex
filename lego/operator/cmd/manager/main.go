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
	"crypto/tls"
	"flag"
	"os"
	"strconv"

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
	"github.com/bex-co/bex/lego/operator/internal/publish"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	bexruntime "github.com/bex-co/bex/lego/operator/internal/runtime"
	bexwebhook "github.com/bex-co/bex/lego/operator/internal/webhook"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// envOr returns the env var k or a default.
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
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

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

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

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	appsNamespace := envOr("BEX_APPS_NAMESPACE", "default")
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Cache:                  controller.NamespacedSecretCacheOptions(appsNamespace),
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "36450c48.bex.co",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
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
	buildClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		setupLog.Error(err, "Failed to create uncached build client")
		os.Exit(1)
	}

	activatorPort := 8888
	if v := os.Getenv("BEX_ACTIVATOR_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			activatorPort = p
		}
	}
	staticServerPort := 8080
	if v := os.Getenv("BEX_STATIC_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			staticServerPort = p
		}
	}
	maxConcurrentBuilds := 0
	if v := os.Getenv("BEX_MAX_CONCURRENT_BUILDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxConcurrentBuilds = n
		}
	}
	appReconciler := &controller.AppReconciler{
		Client:                  mgr.GetClient(),
		BuildClient:             buildClient,
		Scheme:                  mgr.GetScheme(),
		Mode:                    envOr("BEX_RUNTIME", controller.ModeOpenSandbox),
		Registry:                envOr("BEX_REGISTRY", "127.0.0.1:5050"),
		KpackRegistry:           os.Getenv("BEX_KPACK_REGISTRY"),
		CNBBuilder:              envOr("BEX_CNB_BUILDER", "paketobuildpacks/builder-jammy-base"),
		BuildNamespace:          os.Getenv("BEX_BUILD_NAMESPACE"),
		Runtime:                 bexruntime.New(envOr("BEX_OPENSANDBOX_URL", "http://127.0.0.1:8077")),
		BaseDomain:              envOr("BEX_BASE_DOMAIN", ""),
		ClusterIssuer:           envOr("BEX_CLUSTER_ISSUER", "letsencrypt-staging"),
		ActivatorService:        envOr("BEX_ACTIVATOR_SERVICE", ""),
		ActivatorPort:           activatorPort,
		MaintenanceService:      envOr("BEX_ACTIVATOR_SERVICE", "bex-activator"),
		MaintenanceNamespace:    envOr("POD_NAMESPACE", "bex-system"),
		MaintenancePort:         activatorPort,
		MaxConcurrentBuilds:     maxConcurrentBuilds,
		MaxConcurrentReconciles: positiveEnvInt("BEX_APP_RECONCILE_WORKERS", 1),
		StaticStore: publish.Store{
			Bucket:   envOr("BEX_STATIC_S3_BUCKET", ""),
			Endpoint: envOr("BEX_STATIC_S3_ENDPOINT", ""),
			Region:   envOr("BEX_STATIC_S3_REGION", ""),
			Secret:   envOr("BEX_STATIC_S3_SECRET", ""),
		},
		StaticServerService: envOr("BEX_STATIC_SERVER_SERVICE", ""),
		StaticServerPort:    staticServerPort,
		TenantSignKeySecret: os.Getenv("BEX_TENANT_SIGNING_KEY_SECRET"),
		TenantSignImage:     envOr("BEX_TENANT_SIGNING_IMAGE", ""),
		RegistryPushSecret:  os.Getenv("BEX_REGISTRY_PUSH_SECRET"),
		RegistryPullSecret:  os.Getenv("BEX_REGISTRY_PULL_SECRET"),
	}
	// Per-App registry pull credentials (w7/m36). Active when BEX_REGISTRY_NS is
	// set (typically "bex-registry"). Supersedes the shared bex-puller path.
	if zotNS := os.Getenv("BEX_REGISTRY_NS"); zotNS != "" {
		rc := 0
		if v := os.Getenv("BEX_ZOT_RETENTION_COUNT"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				rc = n
			}
		}
		appReconciler.PerAppRegistry = &registry.Creds{
			Client:         mgr.GetClient(),
			ZotNamespace:   zotNS,
			HTPasswdName:   envOr("BEX_ZOT_HTPASSWD_SECRET", "zot-htpasswd"),
			ConfigName:     envOr("BEX_ZOT_CONFIG_SECRET", "zot-config"),
			Registry:       envOr("BEX_REGISTRY", "127.0.0.1:5050"),
			KpackRegistry:  os.Getenv("BEX_KPACK_REGISTRY"),
			RetentionCount: rc,
		}
	}
	if cs != nil {
		appReconciler.MetricsReader = controller.NewMetricsServerReader(cs)
	}
	if err := appReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "service")
		os.Exit(1)
	}

	databaseReconciler := &controller.DatabaseReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
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

	if err := (&controller.KeyValueReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		KvDomain: envOr("BEX_KV_DOMAIN", ""),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "keyvalue")
		os.Exit(1)
	}
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
