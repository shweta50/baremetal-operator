package main

/*
 *      Copyright 2020 Platform9, Inc.
 *      All rights reserved
 */

import (
	"flag"
	"fmt"
	"net/http"
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
	api "github.com/platform9/pf9-addon-operator/pkg/api"
	"github.com/platform9/pf9-addon-operator/pkg/k8s"
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
	//ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "75e2bf59.pf9.io",
	})
	if err != nil {
		//setupLog.Error(err, "unable to start manager")
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.AddonReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Addon"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		//setupLog.Error(err, "unable to create controller", "controller", "Addon")
		log.Error(err, "unable to create controller")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	//go healthCheck(mgr.GetClient())

	http.HandleFunc("/v1/availableaddons", func(w http.ResponseWriter, req *http.Request) {
		api.AvailableAddons(w, req)
	})

	http.HandleFunc("/v1/status", func(w http.ResponseWriter, req *http.Request) {
		api.Status(w, req)
	})

	go http.ListenAndServe("0.0.0.0:8080", nil)

	//setupLog.Info("starting manager")
	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		//setupLog.Error(err, "problem running manager")
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

func healthCheck(cl client.Client) {

	time.Sleep(30 * time.Second)

	healthCheckInterval, _ := getEnv("HEALTHCHECK_INTERVAL_MINS", "5")

	w, err := k8s.New("k8s", cl)
	if err != nil {
		log.Error(err, "while processing package")
		panic("Cannot create client")
	}

	for {
		if err := w.HealthCheck(cl); err != nil {
			log.Error(err, "unable to process addon")
			continue
		}
		time.Sleep(time.Duration(healthCheckInterval) * time.Minute)
	}
}

func getEnv(env, def string) (int, error) {
	value, exists := os.LookupEnv(env)
	if !exists {
		value = def
	}

	return strconv.Atoi(value)
}
