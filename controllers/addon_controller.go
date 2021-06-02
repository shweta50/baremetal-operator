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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/addons"
	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/watch"
)

// AddonReconciler reconciles a Addon object
type AddonReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agent.pf9.io,resources=addons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agent.pf9.io,resources=addons/status,verbs=get;update;patch
//Reconcile loop
func (r *AddonReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	var addon = agentv1.Addon{}
	if err := r.Get(ctx, req.NamespacedName, &addon); err != nil {
		log.Error(err, "unable to fetch Addon config")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if addon.ObjectMeta.Generation == addon.Status.ObservedGeneration {
		log.Infof("Ignoring reconcile due to previous status update: %s", addon.Name)
		return ctrl.Result{}, nil
	}

	operation := getOperation(&addon)
	watch.SetAddonOperation(addon.Spec.Type)

	w, err := addons.New(r.Client)
	if err != nil {
		log.Error(err, "while processing package")
		return ctrl.Result{}, err
	}

	syncErr := w.SyncEvent(&addon, operation)
	if syncErr != nil {
		log.Error(err, "unable to process addon")
	}

	addon.Status.ObservedGeneration = addon.ObjectMeta.Generation
	err = r.Status().Update(ctx, &addon)
	if err != nil {
		log.Error(err, "Unable to update status of Addons object")
		return ctrl.Result{}, err
	}

	// Finalizer should be added even if installation fails, otherwise when the Addon object is deleted,
	// cleanup does not happen correctly. Since the finalizer is not present, the object is deleted
	// right away and we cannot get the spec in the Reconcile loop post delete. Deletion anyways ignores
	// those resources which are not present, so cleaning up a partially installed Addon is not an issue
	if !(operation == "uninstall" && syncErr != nil) {
		log.Infof("Updating finalizer for addon: %s", addon.Name)
		setFinalizer(&addon, operation)
	}

	err = r.Update(ctx, &addon)
	if err != nil {
		log.Error(err, "Unable to update Addons object")
		return ctrl.Result{}, err
	}
	log.Infof("Updated addon: %s", addon.Name)

	return addonerr.ProcessError(syncErr)
}

//SetupWithManager function
func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentv1.Addon{}).
		Complete(r)
}

func setFinalizer(addon *agentv1.Addon, operation string) {
	switch operation {
	case "install":
		log.Infof("Adding finalizer for addon: %s", addon.Name)
		addon.ObjectMeta.Finalizers = []string{"addons.pf9.io"}
	case "uninstall":
		log.Infof("Removing finalizer for addon: %s", addon.Name)
		addon.ObjectMeta.Finalizers = []string{}
	}
}

func getOperation(addon *agentv1.Addon) string {
	operation := ""
	finalizers := len(addon.ObjectMeta.Finalizers)

	if addon.ObjectMeta.DeletionTimestamp.IsZero() {
		operation = "install"
		log.Infof("Installing Addon: %s finalizers: %d", addon.Name, finalizers)
	} else {
		operation = "uninstall"
		if finalizers >= 0 {
			log.Infof("Uninstalling Addons: %s", addon.Name)
		} else {
			log.Infof("Uninstalling Addons: %s no Finalizers found", addon.Name)
		}
	}

	return operation
}
