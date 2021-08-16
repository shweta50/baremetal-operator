package addons

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

	"k8s.io/client-go/rest"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-qbert/sunpike/apiserver/pkg/apis/sunpike/v1alpha2"
)

const (
	addonsNS             = "pf9-addons"
	addonsConfigSecret   = "addon-config"
	envVarDisableSunpike = "DISABLE_SUNPIKE_SYNC"
	envVarQuayRegistry   = "QUAY_REGISTRY"
	envVarK8sRegistry    = "K8S_REGISTRY"
	envVarGcrRegistry    = "GCR_REGISTRY"
	envVarDockerRegistry = "DOCKER_REGISTRY"

	defaultQuayRegistry   = "quay.io"
	defaultK8sRegistry    = "k8s.gcr.io"
	defaultGcrRegistry    = "gcr.io"
	defaultDockerRegistry = ""

	templateQuayRegistry   = "QuayRegistry"
	templateK8sRegistry    = "K8sRegistry"
	templateGcrRegistry    = "GcrRegistry"
	templateDockerRegistry = "DockerRegistry"
)

// AddonClient handles events on the Addon object
type AddonClient struct {
	client client.Client
	ctx    context.Context
}

// New returns new instance of watcher
func New(c client.Client) (*AddonClient, error) {
	return &AddonClient{
		client: c,
		ctx:    context.Background(),
	}, nil
}

//SyncEvent processes new addon event
func (w *AddonClient) SyncEvent(addon *agentv1.Addon, operation string) error {

	log.Infof("Operation: %s", operation)
	switch operation {
	case "install":
		log.Debugf("Installing %s (%s)", addon.Name, addon.Spec.Version)
		return w.install(addon)
	case "uninstall":
		log.Debugf("Uninstalling %s (%s)", addon.Name, addon.Spec.Version)
		return w.uninstall(addon)
	case "upgrade":
		log.Debugf("Upgrading %s (%s)", addon.Name, addon.Spec.Version)
		return w.upgrade(addon)
	}
	return nil
}

func (w *AddonClient) install(addon *agentv1.Addon) error {

	var err error

	if err = w.InstallPkg(addon); err != nil {
		addon.Status.Phase = v1alpha2.AddonPhaseInstallError
		addon.Status.Message = fmt.Sprintf("%s", err)
		addon.Status.Healthy = false
	} else {
		addon.Status.Phase = v1alpha2.AddonPhaseInstalled
		addon.Status.Message = ""
	}

	return err
}

func (w *AddonClient) uninstall(addon *agentv1.Addon) error {

	var err error

	if err = w.UninstallPkg(addon); err != nil {
		addon.Status.Phase = v1alpha2.AddonPhaseUnInstallError
		addon.Status.Message = fmt.Sprintf("%s", err)
	} else {
		addon.Status.Healthy = false
		addon.Status.Phase = v1alpha2.AddonPhaseUnInstalled
		addon.Status.Message = ""
	}

	return err
}

func (w *AddonClient) upgrade(addon *agentv1.Addon) error {

	var err error

	if err = w.UpgradePkg(addon); err != nil {
		//TODO Upgrade case is not being handled yet.
	}

	return err
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
