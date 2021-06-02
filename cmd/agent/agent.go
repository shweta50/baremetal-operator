package main

/*
 *      Copyright 2020 Platform9, Inc.
 *      All rights reserved
 */

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/controllers"
	"github.com/platform9/pf9-addon-operator/pkg/addons"
	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/token"
	"github.com/platform9/pf9-addon-operator/pkg/util"
	"github.com/platform9/pf9-addon-operator/pkg/watch"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	//setupLog = ctrl.Log.WithName("setup")
)

const (
	maxClusterAddonErrCount = 10
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

	ctx, cancel := context.WithCancel(context.Background())
	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error { return watchResources(ctx, ctx.Done()) })

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

	cancel()
	if err := wg.Wait(); err != nil {
		log.Error(err, "while waiting for watchers to terminate....")
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

// watchResources will start a watch on all resources deployed by the operator.
// If any of those resources are manually changed, they are brought back to the original configuration.
func watchResources(ctx context.Context, stopc <-chan struct{}) error {

	// Wait for a while before starting watch on resources,
	// during bootstrap all addons deployed by the operator, we want to avoid those notifications
	time.Sleep(90 * time.Second)
	log.Info("Starting watch controller...")
	cl, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		panic(fmt.Sprintf("Error creating client for watchResources: %s", err))
	}

	w, err := watch.New(ctx, cl)
	if err != nil {
		log.Error(err, "while creating watch")
		panic("Cannot create watch")
	}

	if err := w.Run(stopc); err != nil {
		log.Error(err, "unable to start watch")
	}
	return nil
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
	errCount := 0

	for {
		if err := w.HealthCheck(clusterID, projectID); err != nil {
			log.Errorf("Error in healthcheck: %s", err)

			// In case there is an error reaching the du_fqdn then maintain a count
			// beyond which restart the pod, normally client-go works even if the network
			// goes away for a while and is restored, but in some special cases we had
			// to ensure pod restart, see: PMK-3821
			if addonerr.IsListClusterAddons(err) {
				errCount++

				log.Errorf("List ClusterAddons error count: %d of %d", errCount, maxClusterAddonErrCount)
				if errCount > maxClusterAddonErrCount {
					panic("Error listing ClusterAddon objects from sunpike")
				}
			}
		} else {
			errCount = 0
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
