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
	"flag"
	"os"
	"strconv"

	"github.com/amazeeio/aergia/handlers/idler"
	"github.com/amazeeio/aergia/handlers/unidler"
	prometheusapi "github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/prometheus"
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

	var dryRun bool
	var selectorsFile string
	var skipHitCheck bool
	var cliCron string     // interval for the cli idler.
	var serviceCron string // interval for the service idler.

	var prometheusAddress string
	var prometheusCheckInterval string
	var podCheckInterval int

	var enableCLIIdler bool
	var enableServiceIdler bool

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
		"The cron definition for how often to run the cli idling process.")
	flag.StringVar(&prometheusAddress, "prometheus-endpoint", "http://monitoring-kube-prometheus-prometheus.monitoring.svc:9090",
		"The address for the prometheus endpoint to check against")
	flag.StringVar(&prometheusCheckInterval, "prometheus-interval", "4h",
		"The time range interval for how long to check prometheus for (default: 4h)")
	flag.IntVar(&podCheckInterval, "pod-check-interval", 4,
		"The time range interval for how long to check pod update (default: 4)")
	flag.BoolVar(&skipHitCheck, "skip-hit-check", false,
		"Flag to determine if the idler should check the hit backend or not. If true, this overrides what is in the selectors file.")
	flag.BoolVar(&enableCLIIdler, "enable-cli-idler", true, "Flag to enable cli idler.")
	flag.BoolVar(&enableServiceIdler, "enable-service-idler", true, "Flag to enable service idler.")
	flag.Parse()

	selectorsFile = getEnv("SELECTORS_YAML_FILE", selectorsFile)

	dryRun = getEnvBool("DRY_RUN", dryRun)

	cliCron = getEnv("CLI_CRON", cliCron)
	serviceCron = getEnv("SERVICE_CRON", serviceCron)
	enableServiceIdler = getEnvBool("ENABLE_SERVICE_IDLER", enableServiceIdler)
	enableCLIIdler = getEnvBool("ENABLE_CLI_IDLER", enableCLIIdler)
	podCheckInterval = getEnvInt("POD_CHECK_INTERVAL", podCheckInterval)

	prometheusAddress = getEnv("PROMETHEUS_ADDRESS", prometheusAddress)
	prometheusCheckInterval = getEnv("PROMETHEUS_CHECK_INTERVAL", prometheusCheckInterval)

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
	selectors := &idler.IdlerData{}
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

	h := &unidler.Client{
		Client:          mgr.GetClient(),
		Log:             ctrl.Log.WithName("controllers-http").WithName("Unidler"),
		RefreshInterval: refreshInterval,
		Debug:           debug,
		RequestCount:    requestCount,
		RequestDuration: requestDuration,
	}

	prometheusClient, err := prometheusapi.NewClient(prometheusapi.Config{
		Address: prometheusAddress,
	})
	if err != nil {
		setupLog.Error(err, "error creating prometheus client")
		os.Exit(1)
	}

	// setup the handler with the k8s and lagoon clients
	handler := &idler.IdlerHandler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log,
		PodCheckInterval:        podCheckInterval,
		PrometheusClient:        prometheusClient,
		PrometheusCheckInterval: prometheusCheckInterval,
		DryRun:                  dryRun,
		Debug:                   debug,
		Selectors:               selectors,
	}

	// Set up the cron job intervals for the CLI and service idlers.
	c := cron.New()

	// add the cronjobs we need.
	// CLI Idler
	if enableCLIIdler {
		c.AddFunc(cliCron, func() {
			handler.CLIIdler()
		})
	}
	// Service Idler
	if enableServiceIdler {
		c.AddFunc(serviceCron, func() {
			handler.ServiceIdler()
		})
	}
	// start crons.
	c.Start()

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting unidler listening")
	go unidler.Run(h, setupLog)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		podInterval, err := strconv.Atoi(value)
		if err != nil {
			return podInterval
		}
	}
	return fallback
}

// accepts fallback values 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False
// anything else is false.
func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		rVal, _ := strconv.ParseBool(value)
		return rVal
	}
	return fallback
}
