package main

/*
 *      Copyright 2020 Platform9, Inc.
 *      All rights reserved
 */

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/controllers"
	"github.com/platform9/pf9-addon-operator/pkg/addons"
	"github.com/platform9/pf9-addon-operator/pkg/token"
	"github.com/platform9/pf9-addon-operator/pkg/util"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	//setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = agentv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var enableLeaderElection bool
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()
	log.SetFormatter(&log.JSONFormatter{})
	setLogLevel()
	log.Info("Running update ca certs")
	if err := util.UpdateCACerts(); err != nil {
		log.Error(err, " running update ca certs")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "75e2bf59.pf9.io",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.AddonReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Addon"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder
	go healthCheck()

	/*
		TODO: These APIs will soon be required for listing addons available for install
		and version upgrades available for each addon. The following is sample legacy code
		which was added in the previous design.

		http.HandleFunc("/v1/availableaddons", func(w http.ResponseWriter, req *http.Request) {
			api.AvailableAddons(w, req)
		})

		http.HandleFunc("/v1/status", func(w http.ResponseWriter, req *http.Request) {
			api.Status(w, req)
		})

		go http.ListenAndServe("0.0.0.0:8090", nil)
	*/

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setLogLevel() {

	switch lvl := os.Getenv("LOGLEVEL"); lvl {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "INFO":
		break
	case "WARN":
		log.SetLevel(log.WarnLevel)
	case "FATAL":
		log.SetLevel(log.FatalLevel)
	default:
		panic(fmt.Sprintf("Invalid log level: %s", lvl))
	}
}

func healthCheck() {

	cl, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		panic(fmt.Sprintf("Error creating client for healthcheck: %s", err))
	}

	time.Sleep(30 * time.Second)

	healthCheckInterval, _ := getEnvInt("HEALTHCHECK_INTERVAL_SECS", "150")
	clusterID := getEnvUUID("CLUSTER_ID")
	projectID := getEnvUUID("PROJECT_ID")

	w, err := addons.New(cl)
	if err != nil {
		log.Error(err, "while processing package")
		panic("Cannot create client")
	}

	for {
		if err := w.HealthCheck(clusterID, projectID); err != nil {
			log.Error(err, "unable to health check addons")
		}
		time.Sleep(time.Duration(healthCheckInterval) * time.Second)
	}
}

func getEnvInt(env, def string) (int, error) {
	value, exists := os.LookupEnv(env)
	if !exists {
		value = def
	}

	return strconv.Atoi(value)
}

func getEnvUUID(env string) string {
	value, exists := os.LookupEnv(env)
	if !exists {
		panic(fmt.Sprintf("%s not defined as env variable", env))
	}

	if !token.IsValidUUID(value) {
		panic(fmt.Sprintf("Invalid UUID: %s", env))
	}

	return value
}
