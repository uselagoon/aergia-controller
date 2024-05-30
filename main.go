/*

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
	"flag"
	"fmt"
	"os"
	"time"

	prometheusapi "github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/uselagoon/aergia-controller/controllers"
	"github.com/uselagoon/aergia-controller/handlers/idler"
	"github.com/uselagoon/aergia-controller/handlers/metrics"
	"github.com/uselagoon/aergia-controller/handlers/unidler"
	u "github.com/uselagoon/aergia-controller/handlers/unidler"
	variables "github.com/uselagoon/machinery/utils/variables"
	"gopkg.in/robfig/cron.v2"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(requestDuration)
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var debug bool
	var refreshInterval int
	var unidlerHTTPPort int

	var dryRun bool
	var selectorsFile string
	var skipHitCheck bool
	var cliCron string     // interval for the cli idler.
	var serviceCron string // interval for the service idler.

	var prometheusAddress string
	var prometheusCheckInterval string
	var podCheckInterval string

	var enableCLIIdler bool
	var enableServiceIdler bool
	var verifiedUnidling bool
	var verifiedSecret string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&debug, "debug", false, "Enable more verbose debug logs.")
	flag.IntVar(&refreshInterval, "refresh-interval", 30,
		"The default refresh interval for the unidle page to use when unidling a namespace.")
	flag.BoolVar(&dryRun, "dry-run", false,
		"Dry run will not perform any idling, just log the actions it would take.")
	flag.StringVar(&selectorsFile, "selectors", "resources/selectors.yaml",
		"The path to the file containing the label selectors for idling.")
	flag.StringVar(&cliCron, "cli-idler-cron", "5,35 * * * *",
		"The cron definition for how often to run the cli idling process.")
	flag.StringVar(&serviceCron, "service-idler-cron", "0 */4 * * *",
		"The cron definition for how often to run the service idling process.")
	flag.StringVar(&prometheusAddress, "prometheus-endpoint", "http://monitoring-kube-prometheus-prometheus.monitoring.svc:9090",
		"The address for the prometheus endpoint to check against")
	flag.StringVar(&prometheusCheckInterval, "prometheus-interval", "4h",
		"The time range interval for how long to check prometheus for (default: 4h)")
	flag.StringVar(&podCheckInterval, "pod-check-interval", "4h",
		"The time range interval for how long to check pod update (default: 4h)")
	flag.BoolVar(&skipHitCheck, "skip-hit-check", false,
		"Flag to determine if the idler should check the hit backend or not. If true, this overrides what is in the selectors file.")
	flag.BoolVar(&enableCLIIdler, "enable-cli-idler", true, "Flag to enable cli idler.")
	flag.BoolVar(&enableServiceIdler, "enable-service-idler", true, "Flag to enable service idler.")
	flag.BoolVar(&verifiedUnidling, "verified-unidling", false,
		"Flag to enable unidling requests to verify that they are a browser by having the first request do a callback to Aergia with verification.")
	flag.StringVar(&verifiedSecret, "verify-secret", "super-secret-string",
		"The secret to use for verifying unidling requests.")
	flag.IntVar(&unidlerHTTPPort, "unidler-port", 5000, "Port for the unidler service to listen on.")
	flag.Parse()

	selectorsFile = variables.GetEnv("SELECTORS_YAML_FILE", selectorsFile)
	verifiedUnidling = variables.GetEnvBool("VERIFIED_UNIDLING", verifiedUnidling)
	verifiedSecret = variables.GetEnv("VERIFY_SECRET", verifiedSecret)

	dryRun = variables.GetEnvBool("DRY_RUN", dryRun)

	unidlerHTTPPort = variables.GetEnvInt("UNIDLER_PORT", unidlerHTTPPort)
	cliCron = variables.GetEnv("CLI_CRON", cliCron)
	serviceCron = variables.GetEnv("SERVICE_CRON", serviceCron)
	enableServiceIdler = variables.GetEnvBool("ENABLE_SERVICE_IDLER", enableServiceIdler)
	enableCLIIdler = variables.GetEnvBool("ENABLE_CLI_IDLER", enableCLIIdler)
	podCheckInterval = variables.GetEnv("POD_CHECK_INTERVAL", podCheckInterval)
	timePodCheckInterval, err := time.ParseDuration(podCheckInterval)
	if err != nil {
		// if the first parse fails, it may be because the user is using a single integer hour value from a previous release
		// this handles the conversion from the previous integer value to the new time.Duration value support.
		timePodCheckInterval, err = time.ParseDuration(fmt.Sprintf("%sh", podCheckInterval))
		if err != nil {
			setupLog.Error(err, "unable to decode pod check interval")
			os.Exit(1)
		}
	}

	prometheusAddress = variables.GetEnv("PROMETHEUS_ADDRESS", prometheusAddress)
	prometheusCheckInterval = variables.GetEnv("PROMETHEUS_CHECK_INTERVAL", prometheusCheckInterval)
	timePrometheusCheckInterval, err := time.ParseDuration(prometheusCheckInterval)
	if err != nil {
		setupLog.Error(err, "unable to decode prometheus check interval")
		os.Exit(1)
	}
	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	// read the selector file into idlerdata struct.
	file, err := os.Open(selectorsFile)
	if err != nil {
		setupLog.Error(err, "unable to open selectors file")
		os.Exit(1)
	}
	defer file.Close()
	d := yaml.NewDecoder(file)
	selectors := &idler.Data{}
	if err := d.Decode(&selectors); err != nil {
		setupLog.Error(err, "unable to decode selectors yaml")
		os.Exit(1)
	}

	if skipHitCheck {
		selectors.Service.SkipHitCheck = skipHitCheck
	}

	if dryRun {
		setupLog.Info("dry-run enabled")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "aergia-unidler-leader-election-helper",
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// if a blockedagents file is found, provide them to the unidler to block agents from unidling environments
	// provides nil if no file found
	allowedAgents, _ := unidler.ReadSliceFromFile("/lists/allowedagents")
	blockedAgents, _ := unidler.ReadSliceFromFile("/lists/blockedagents")
	allowedIPs, _ := unidler.ReadSliceFromFile("/lists/allowedips")
	blockedIPs, _ := unidler.ReadSliceFromFile("/lists/blockedips")
	unidler := &unidler.Unidler{
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("aergia-controller").WithName("Unidler"),
		RefreshInterval:   refreshInterval,
		Debug:             debug,
		RequestCount:      requestCount,
		RequestDuration:   requestDuration,
		AllowedUserAgents: allowedAgents,
		BlockedUserAgents: blockedAgents,
		AllowedIPs:        allowedIPs,
		BlockedIPs:        blockedIPs,
		UnidlerHTTPPort:   unidlerHTTPPort,
		VerifiedUnidling:  verifiedUnidling,
		VerifiedSecret:    verifiedSecret,
	}

	prometheusClient, err := prometheusapi.NewClient(prometheusapi.Config{
		Address: prometheusAddress,
	})
	if err != nil {
		setupLog.Error(err, "error creating prometheus client")
		os.Exit(1)
	}

	// setup the idler with the k8s and lagoon clients
	idler := &idler.Idler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName("aergia-controller").WithName("ServiceIdler"),
		PodCheckInterval:        timePodCheckInterval,
		PrometheusClient:        prometheusClient,
		PrometheusCheckInterval: timePrometheusCheckInterval,
		DryRun:                  dryRun,
		Debug:                   debug,
		Selectors:               selectors,
	}

	// Set up the cron job intervals for the CLI and service idlers.
	c := cron.New()

	// add the cronjobs we need.
	// CLI Idler
	if enableCLIIdler {
		setupLog.Info("starting cli idler")
		c.AddFunc(cliCron, func() {
			idler.CLIIdler()
		})
	}
	// Service Idler
	if enableServiceIdler {
		setupLog.Info("starting service idler")
		c.AddFunc(serviceCron, func() {
			idler.ServiceIdler()
		})
	}
	// start crons.
	c.Start()

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting unidler listening")
	go u.Run(unidler, setupLog)

	if err = (&controllers.IdlingReconciler{
		Client:  mgr.GetClient(),
		Log:     ctrl.Log.WithName("controllers").WithName("Idling"),
		Scheme:  mgr.GetScheme(),
		Idler:   idler,
		Unidler: unidler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Idling")
		os.Exit(1)
	}

	setupLog.Info("starting aergia metrics server")
	m := metrics.NewServer(setupLog, ":9912")
	defer m.Shutdown(context.Background())

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
