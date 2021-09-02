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
	"github.com/platform9/pf9-addon-operator/pkg/util"
	"github.com/platform9/pf9-addon-operator/pkg/watch"
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

	util.CheckEnvVarsOnBootup()

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

	ctx, cancel := context.WithCancel(context.Background())
	wg, ctx := errgroup.WithContext(ctx)

	wg.Go(func() error { return healthCheck(ctx, ctx.Done()) })
	wg.Go(func() error { return watchResources(ctx, ctx.Done()) })

	log.Info("Starting manager...")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}

	cancel()
	if err := wg.Wait(); err != nil {
		log.Error(err, "while waiting for watchers to terminate....")
	}
	log.Info("Exiting gracefully...")
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

	disableWatch := os.Getenv(util.DisableWatchEnvVar)
	if disableWatch == util.DisableWatchVal {
		return nil
	}

	watchInterval := util.GetWatchSleep()

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

	ticker := time.NewTicker(time.Second * time.Duration(watchInterval))
	for {
		select {
		case <-ticker.C:
			if err := w.Run(); err != nil {
				log.Error(err, "unable to start watch")
			}
		case <-stopc:
			log.Info("quiting watchResources")
			return nil
		}
	}
}

func healthCheck(ctx context.Context, stopc <-chan struct{}) error {

	disableSync := os.Getenv(util.DisableSunpikeEnvVar)
	if disableSync == util.DisableSunpikeVal {
		return nil
	}

	cl, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		panic(fmt.Sprintf("Error creating client for healthcheck: %s", err))
	}

	// Wait for a few secs, to allow the cluster to settle down, this is relevant esp during bootstrap
	// when all components are not yet up, it doesn't matter if this value is too less and the sync fails
	// it will keep on trying every healthCheckInterval secs till the values converge
	time.Sleep(30 * time.Second)

	log.Info("Starting sunpike sync...")
	healthCheckInterval := util.GetHealthCheckSleep()
	maxSyncErrCount := util.GetSyncErrorCount()

	clusterID := os.Getenv(util.ClusterIDEnvVar)
	projectID := os.Getenv(util.ProjectIDEnvVar)

	w, err := addons.New(cl)
	if err != nil {
		log.Error(err, "while processing package")
		panic("Cannot create client")
	}
	errCount := 0

	ticker := time.NewTicker(time.Second * time.Duration(healthCheckInterval))
	for {
		select {
		case <-ticker.C:
			if err := w.HealthCheck(ctx, clusterID, projectID); err != nil {
				log.Errorf("Error in healthcheck: %s", err)

				// In case there is an error reaching the du_fqdn then maintain a count
				// beyond which restart the pod, normally client-go works even if the network
				// goes away for a while and is restored, but in some special cases we had
				// to ensure pod restart, see: PMK-3821
				if addonerr.IsListClusterAddons(err) || addonerr.IsGenKeystoneToken(err) {
					errCount++

					log.Errorf("List ClusterAddons error count: %d of %d", errCount, maxSyncErrCount)
					if errCount > maxSyncErrCount {
						panic("Error listing ClusterAddon objects from sunpike")
					}
				}
			} else {
				errCount = 0
			}

		case <-stopc:
			log.Info("quiting healthCheck")
			return nil
		}
	}
}