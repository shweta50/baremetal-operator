package k8s

/*
 Copyright [2020] [Platform9 Systems, Inc]

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

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	"k8s.io/client-go/rest"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	//	v1 "k8s.io/api/core/v1"
	//v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/objects"
)

// Watcher watches for changes in kubernetes events
type Watcher struct {
	c      client.Client
	client kubernetes.Interface
	lock   sync.Mutex
}

// New returns new instance of watcher
func New(mode string, c client.Client) (*Watcher, error) {
	var cfg *rest.Config
	var err error

	switch mode {
	case "standalone":
		cfg, err = getByKubeCfg()
	case "k8s":
		cfg, err = getInCluster()
	default:
		return nil, errors.New("Invalid mode")
	}

	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating kubernetes client")
	}

	return &Watcher{
		c:      c,
		client: client,
		lock:   sync.Mutex{},
	}, nil
}

// ListAddons lists all available addons and their status
func (w *Watcher) ListAddons() ([]objects.AddonState, error) {

	var currState []objects.AddonState

	addonList := &agentv1.AddonList{}
	err := w.c.List(context.Background(), addonList)
	if err != nil {
		log.Error("failed to list addons", err)
		return nil, err
	}

	for _, a := range addonList.Items {
		currState = append(currState, objects.AddonState{
			Name:         a.Name,
			Version:      a.Spec.Version,
			Type:         a.Spec.Type,
			CurrentState: a.Status.CurrentState,
		})
	}

	return currState, nil
}

//HealthCheck checks health of all installed addons
func (w *Watcher) HealthCheck(cl client.Client) error {

	addonList := &agentv1.AddonList{}
	err := cl.List(context.Background(), addonList)
	if err != nil {
		log.Error("failed to list addons", err)
		return err
	}

	for _, a := range addonList.Items {

		if a.Status.CurrentState != "installed" {
			continue
		}

		healthy := "false"
		log.Debugf("Checking health of: %s/%s", a.Namespace, a.Name)

		/*healthy, err := w.checkDeploy(a)
		if err != nil {
			log.Error("failed to check deploy", err)
			return err
		}*/

		if a.Status.Healthy == healthy {
			continue
		}

		log.Infof("Setting health for addon: %s/%s with %s", a.Namespace, a.Name, healthy)
		a.Status.Healthy = healthy
		if err = cl.Status().Update(context.Background(), &a); err != nil {
			log.Error("failed to update addon status", err)
			continue
		}
	}

	return nil
}

//SyncEvent processes new addon event
func (w *Watcher) SyncEvent(addon *agentv1.Addon) error {

	log.Infof("Operation: %s", addon.Spec.Operation)
	switch addon.Spec.Operation {
	case "install":
		log.Debugf("Installing %s (%s)", addon.Name, addon.Spec.Version)
		w.install(addon)
	case "uninstall":
		log.Debugf("Uninstalling %s (%s)", addon.Name, addon.Spec.Version)
		w.uninstall(addon)
	case "upgrade":
		log.Debugf("Upgrading %s (%s)", addon.Name, addon.Spec.Version)
		w.upgrade(addon)
	}
	return nil
}

func (w *Watcher) install(addon *agentv1.Addon) error {

	addon.Status.LastOperation = "install"

	if err := w.InstallPkg(addon); err != nil {
		addon.Status.Healthy = "false"
		addon.Status.LastOperationResult = "failure"
		addon.Status.LastOperationMessage = fmt.Sprintf("%s", err)
	} else {
		addon.Status.Healthy = "true"
		addon.Status.CurrentState = "installed"
		addon.Status.LastOperationResult = "success"
		addon.Status.LastOperationMessage = ""
	}

	return nil
}

func (w *Watcher) uninstall(addon *agentv1.Addon) error {

	addon.Status.LastOperation = "uninstall"

	if err := w.UninstallPkg(addon); err != nil {
		addon.Status.LastOperationResult = "failure"
		addon.Status.LastOperationMessage = fmt.Sprintf("%s", err)
	} else {
		addon.Status.Healthy = "false"
		addon.Status.CurrentState = "uninstalled"
		addon.Status.LastOperationResult = "success"
		addon.Status.LastOperationMessage = ""
	}

	return nil
}

func (w *Watcher) upgrade(addon *agentv1.Addon) error {

	addon.Status.LastOperation = "upgrade"
	if err := w.UpgradePkg(addon); err != nil {
		addon.Status.LastOperationResult = "failure"
		addon.Status.LastOperationMessage = fmt.Sprintf("%s", err)
	} else {
		addon.Status.Healthy = "true"
		addon.Status.CurrentState = "installed"
		addon.Status.LastOperationResult = "success"
		addon.Status.LastOperationMessage = ""
	}

	return nil
}

func getInCluster() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return cfg, err
}

func getByKubeCfg() (*rest.Config, error) {
	defaultKubeCfg := path.Join(os.Getenv("HOME"), ".kube", "config")

	if os.Getenv("KUBECONFIG") != "" {
		defaultKubeCfg = os.Getenv("KUBECONFIG")
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", defaultKubeCfg)
	if err != nil {
		return nil, errors.Wrap(err, "building kubecfg")
	}

	return cfg, err
}
