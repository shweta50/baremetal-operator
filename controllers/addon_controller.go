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
	"github.com/platform9/pf9-addon-operator/pkg/k8s"
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

	//log := r.Log.WithValues("addon", req.NamespacedName)
	var addon = agentv1.Addon{}
	if err := r.Get(ctx, req.NamespacedName, &addon); err != nil {
		log.Error(err, "unable to fetch Addon config")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	//log.Debugf("Addon operation: %s State: %s", addon.Spec.Operation, addon.Status.CurrentState)
	//log.Debugf("Generation metadata: %d status: %d", addon.ObjectMeta.Generation, addon.Status.ObservedGeneration)

	if addon.ObjectMeta.Generation == addon.Status.ObservedGeneration {
		//log.Debug("Ignoring status update")
		return ctrl.Result{}, nil
	}

	w, err := k8s.New("k8s", r.Client)
	if err != nil {
		log.Error(err, "while processing package")
		return ctrl.Result{}, err
	}

	if err := w.SyncEvent(&addon); err != nil {
		log.Error(err, "unable to process addon")
		return ctrl.Result{}, err
	}

	addon.Status.ObservedGeneration = addon.ObjectMeta.Generation
	if err := r.Status().Update(ctx, &addon); err != nil {
		log.Error(err, "unable to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

//SetupWithManager function
func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentv1.Addon{}).
		Complete(r)
}
