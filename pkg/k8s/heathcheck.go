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
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	log "github.com/sirupsen/logrus"

	//	v1 "k8s.io/api/core/v1"
	//v1 "k8s.io/api/core/v1"

	//"k8s.io/client-go/kubernetes"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/token"
	"github.com/platform9/pf9-qbert/sunpike/apiserver/pkg/apis/sunpike/v1alpha2"
	clientset "github.com/platform9/pf9-qbert/sunpike/apiserver/pkg/generated/clientset/versioned"
)

func (w *Watcher) getClusterAddons(kubeCfg *rest.Config, clusterID, projectID string) (map[string]v1alpha2.ClusterAddon, error) {

	//Get all ClusterAddon objects from sunpike and store in a map
	mapClsAddon := map[string]v1alpha2.ClusterAddon{}

	// Create Sunpike client
	sunpikeClient, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		return mapClsAddon, err
	}

	//Get all ClusterAddon objects from sunpike apiserver
	listOptions := metav1.ListOptions{
		LabelSelector: "clusterid=" + clusterID,
	}
	clsAddonList, err := sunpikeClient.SunpikeV1alpha2().ClusterAddons().List(w.ctx, listOptions)
	if err != nil {
		log.Error(err, "Unable to list ClusterAddons")
		return mapClsAddon, err
	}

	for _, clsAddon := range clsAddonList.Items {
		//log.Debugf("Adding cls addon: %s", clsAddon.Name)
		mapClsAddon[clsAddon.Name] = clsAddon
	}

	return mapClsAddon, nil
}

func (w *Watcher) getAddons() (map[string]agentv1.Addon, error) {

	//Get all Addon objects from local cluster and store in a map
	mapAddon := map[string]agentv1.Addon{}

	addonList := &agentv1.AddonList{}
	err := w.cl.List(w.ctx, addonList)
	if err != nil {
		log.Error("Failed to list addons", err)
		return mapAddon, err
	}

	for _, a := range addonList.Items {
		//log.Debugf("Adding addon: %s", a.Name)
		mapAddon[a.Name] = a
	}
	return mapAddon, nil
}

func updateAddon(from *agentv1.Addon, to *agentv1.Addon) {
	to.Spec.Version = from.Spec.Version
	to.Spec.Type = from.Spec.Type
	to.Spec.Watch = from.Spec.Watch
	to.Spec.Override.Params = from.Spec.Override.Params
}

func convertToAddon(clsAddon *v1alpha2.ClusterAddon) agentv1.Addon {
	addon := agentv1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "pf9-addons",
			Name:      clsAddon.Name,
		},
		Spec: agentv1.AddonSpec{
			Version: clsAddon.Spec.Version,
			Type:    clsAddon.Spec.Type,
			Watch:   clsAddon.Spec.Watch,
		},
	}

	if len(clsAddon.Spec.Override.Params) > 0 {
		addon.Spec.Override.Params = []agentv1.Params{}
		for _, p := range clsAddon.Spec.Override.Params {
			param := agentv1.Params{
				Name:  p.Name,
				Value: p.Value,
			}
			addon.Spec.Override.Params = append(addon.Spec.Override.Params, param)
		}
	}

	return addon
}

//HealthCheck checks health of all installed addons
func (w *Watcher) HealthCheck(clusterID, projectID string) error {

	if err := w.updateHealth(); err != nil {
		return err
	}

	if err := w.syncClusterAddons(clusterID, projectID); err != nil {
		return err
	}

	return nil
}

//sync the state of ClusterAddon objects on sunpike with local Addon objects
func (w *Watcher) syncClusterAddons(clusterID, projectID string) error {

	//Get token and sunpike kubeconfig for this cluster
	/*ksToken, err := token.GetKsToken(projectID)
	if err != nil {
		return nil
	}*/
	ksToken := ""
	kubeCfg, err := token.GetSunpikeKubeCfg(ksToken, clusterID, projectID)
	if err != nil {
		log.Errorf("Unable to get kubeconfig for cluster: %s %s", clusterID, err)
		return err
	}

	//Store ClusterAddon objects in a map
	mapClsAddon, err := w.getClusterAddons(kubeCfg, clusterID, projectID)
	if err != nil {
		log.Errorf("Unable to get ClusterAddon objects for cluster: %s %s", clusterID, err)
		return err
	}

	//Store Addon objects in a map
	mapAddon, err := w.getAddons()
	if err != nil {
		log.Errorf("Unable to get Addon objects %s", err)
		return err
	}

	//For each ClusterAddon object check if local Addon objects have changed
	//Create/Update/Delete Addon objects locally when any diff is detected
	//Also update ClusterAddon object status if Addon object is deleted
	for _, clsAddon := range mapClsAddon {
		localAddon, ok := mapAddon[clsAddon.Name]
		w.processClusterAddon(kubeCfg, &clsAddon, &localAddon, ok)
	}

	//In case of a diff is detected between status of local Addon object and
	//ClusterAddon object: update status of ClusterAddon object
	for _, addon := range mapAddon {
		if spClsAddon, ok := mapClsAddon[addon.Name]; ok {
			w.processAddon(kubeCfg, addon, spClsAddon)
		}
	}

	return nil
}

func (w *Watcher) processAddon(kubeCfg *rest.Config, localAddon agentv1.Addon, clsAddon v1alpha2.ClusterAddon) error {

	if localAddon.Status.CurrentState == clsAddon.Status.CurrentState &&
		localAddon.Status.Healthy == clsAddon.Status.Healthy {
		log.Debugf("Not updating ClusterAddon status: %s", clsAddon.Name)
		return nil
	}

	clsAddon.Status.CurrentState = localAddon.Status.CurrentState
	clsAddon.Status.Healthy = localAddon.Status.Healthy

	log.Infof("Updating ClusterAddon object: %s status with %s", clsAddon.Name, clsAddon.Status.CurrentState)
	if err := w.updateSunpikeStatus(kubeCfg, &clsAddon); err != nil {
		log.Errorf("Failed to update ClusterAddon status: %s %s", clsAddon.Name, err)
	}

	return nil
}

func (w *Watcher) updateSunpikeStatus(kubeCfg *rest.Config, clsAddon *v1alpha2.ClusterAddon) error {
	// Create Sunpike client
	sunpikeClient, err := clientset.NewForConfig(kubeCfg)
	if err != nil {
		return err
	}

	_, err = sunpikeClient.SunpikeV1alpha2().ClusterAddons().Update(w.ctx, clsAddon, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Failed to update cls addon status: %s %s", clsAddon.Name, err)
		return err
	}

	return nil
}

func (w *Watcher) processClusterAddon(kubeCfg *rest.Config, clsAddon *v1alpha2.ClusterAddon, localAddon *agentv1.Addon, ok bool) error {
	//Convert clusterAddon object to Addon
	convAddon := convertToAddon(clsAddon)

	//Check if ClusterAddon is being deleted
	if !clsAddon.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Infof("Deleting Addon object: %s", clsAddon.Name)
		if ok {
			//Delete local Addon object
			err := w.cl.Delete(w.ctx, &convAddon)
			if err != nil {
				log.Errorf("Failed to delete addon: %s %s", clsAddon.Name, err)
			}
		} else {
			//Local Addon is already deleted and sunpike ClusterAddon is in deleting state
			//This means successfull uninstallation has already happened previously of local Addon
			//Update ClusterAddon status accordingly
			if clsAddon.Status.CurrentState == "uninstall-success" {
				return nil
			}

			clsAddon.Status.CurrentState = "uninstall-success"
			clsAddon.Status.Healthy = false
			log.Infof("Updating ClusterAddon object: %s status with %s", clsAddon.Name, clsAddon.Status.CurrentState)
			if err := w.updateSunpikeStatus(kubeCfg, clsAddon); err != nil {
				log.Errorf("Failed to update sunpike status for deleted addon: %s %s", clsAddon.Name, err)
			}
		}
		return nil
	}

	//Check if we need to create Addon for newly created ClusterAddon
	if !ok {
		//Create Addon object
		log.Infof("Creating Addon object: %s", clsAddon.Name)
		err := w.cl.Create(w.ctx, &convAddon)
		if err != nil {
			log.Errorf("Failed to create addon: %s %s", clsAddon.Name, err)
		}
		return nil
	}

	//Check if ClusterAddon has changed from the local Addon
	if reflect.DeepEqual(convAddon.Spec, localAddon.Spec) {
		log.Debugf("Not updating addon object: %s", clsAddon.Name)
		return nil
	}

	log.Infof("Updating Addon object: %s", clsAddon.Name)
	updateAddon(&convAddon, localAddon)
	err := w.cl.Update(w.ctx, localAddon)
	if err != nil {
		log.Errorf("Failed to update addon: %s %s", clsAddon.Name, err)
	}

	return nil
}

//Check health of each installed Addon and set status healthy=true
//Update status only if it has changed
func (w *Watcher) updateHealth() error {
	addonList := &agentv1.AddonList{}
	err := w.cl.List(w.ctx, addonList)
	if err != nil {
		log.Error("failed to list addons", err)
		return err
	}

	for _, a := range addonList.Items {
		if !strings.HasPrefix(a.Status.CurrentState, "install-success") {
			continue
		}

		healthy := false
		//log.Debugf("Checking health of: %s/%s", a.Namespace, a.Name)

		addonClient := getAddonClient(a.Spec.Type, a.Spec.Version, nil, w.cl)

		if healthy, err = addonClient.Health(); err != nil {
			log.Errorf("Error getting health of: %s %s", a.Name, err)
			return err
		}

		if a.Status.Healthy == healthy {
			continue
		}

		log.Infof("Setting health for addon: %s/%s with %t", a.Namespace, a.Name, healthy)
		a.Status.Healthy = healthy
		if err = w.cl.Status().Update(w.ctx, &a); err != nil {
			log.Error("failed to update addon status", err)
			continue
		}
	}

	return nil
}
