package watch

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-qbert/sunpike/apiserver/pkg/apis/sunpike/v1alpha2"
)

const (
	resourceFile = "/etc/addon/resources.yaml"
	waitCount    = 20
	waitSecs     = 5
	addonNs      = "pf9-addons"
)

// Watch watches resources deployed by addon operator
type Watch struct {
	ctx    context.Context
	client client.Client

	checkRes        map[string]string
	addonResVersion map[string]int
}

// Config is a map of resources we are expected to watch
type Config struct {
	Resources map[string]string `yaml:"resources"`
}

// New returns new instance of watcher
func New(ctx context.Context, cl client.Client) (*Watch, error) {

	checkRes, err := readResourcesFile()
	if err != nil {
		return nil, err
	}

	log.Infof("Watch: Read resources file, %d entries found", len(checkRes))

	return &Watch{
		ctx:             ctx,
		client:          cl,
		checkRes:        checkRes,
		addonResVersion: map[string]int{},
	}, nil
}

func readResourcesFile() (map[string]string, error) {
	config := Config{}

	content, err := ioutil.ReadFile(resourceFile)
	if err != nil {
		log.Errorf("Failed to read resources file %s %s", resourceFile, err)
		return nil, err
	}

	if err = yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	return config.Resources, nil
}

// Run starts sync workers
func (w *Watch) Run() error {
	log.Info("Running watch...")
	mapAddon := map[string]string{}

	addonList := &agentv1.AddonList{}
	err := w.client.List(w.ctx, addonList)
	if err != nil {
		log.Error("Failed to list addons", err)
		return err
	}

	for _, a := range addonList.Items {
		// Watch resources of only those addons which are installed and want to be watched
		if a.Status.Phase == v1alpha2.AddonPhaseInstalled && a.Spec.Watch {
			log.Debugf("Watch: Listing addon: %s", a.Name)
			mapAddon[a.Spec.Type] = a.Name
		}
	}

	for res, addonType := range w.checkRes {
		if addonName, ok := mapAddon[addonType]; ok {
			log.Debugf("Watch: Checking resource: %s", res)
			currentResVersion, err := w.getObject(res)
			if err != nil {
				log.Errorf("Unable to get object: %s %s", res, err)
				return err
			}

			expectedResVersion, ok := w.addonResVersion[res]
			log.Debugf("Watch: resource version expected: %d, current: %d", expectedResVersion, currentResVersion)

			if ok {
				if currentResVersion > expectedResVersion {
					log.Infof("Watch: resource: %s has changed, triggering addon: %s", res, addonName)
					w.triggerAddon(addonName)
				}
			} else {
				log.Infof("Watch: addon Version not found for: %s in cache, triggering addon: %s", res, addonName)
				if err = w.triggerAddon(addonName); err != nil {
					log.Errorf("Unable to trigger addon: %s %s", addonName, err)
					return err
				}
			}
		}
	}

	return nil
}

func (w *Watch) waitforAddon(name string, observedGeneration int64) error {
	addon := &agentv1.Addon{}

	for i := 0; i < waitCount; i++ {
		time.Sleep(waitSecs * time.Second)

		err := w.client.Get(w.ctx, types.NamespacedName{Name: name, Namespace: addonNs}, addon)
		if err != nil {
			log.Errorf("Watch: Error waiting for Addon: %s, %s", name, err)
			continue
		} else if addon.Status.ObservedGeneration == observedGeneration {
			log.Infof("Watch: Addon %s converged after triggering it", name)
			return nil
		}
		log.Infof("Watch: Addon %s waiting to converge: %d %d", name, addon.Status.ObservedGeneration, observedGeneration)
	}

	return fmt.Errorf("Watch: Addon %s did not converge after triggering it", name)
}

func (w *Watch) triggerAddon(name string) error {
	addon := &agentv1.Addon{}
	err := w.client.Get(w.ctx, types.NamespacedName{Name: name, Namespace: addonNs}, addon)
	if err != nil {
		log.Errorf("Failed to get addon: %s %s", name, err)
		return err
	}

	// The addon reconcile loop interprets a valid change in Addon spec when
	// generation number > observed generation number. For e.g. when the health check loop updates
	// the status of the Addon object, the generation number is not incremented, and that's how
	// the Addon Reconcile loop knows that this trigger should be ignored. Using that here, to
	// trigger off a reconcile for the Addon, by reducing the observed generation number.
	// See the Reconcile loop and how it uses observed generation number for more details.
	observedGeneration := addon.Status.ObservedGeneration
	addon.Status.ObservedGeneration = observedGeneration - 1
	err = w.client.Status().Update(w.ctx, addon)
	if err != nil {
		log.Errorf("Watch: Failed to update addon: %s %s", name, err)
		return err
	}

	if err = w.waitforAddon(name, observedGeneration); err != nil {
		return err
	}

	for res, addonType := range w.checkRes {
		if addonType != addon.Spec.Type {
			continue
		}

		currentResVersion, err := w.getObject(res)
		if err != nil {
			continue
		}

		w.addonResVersion[res] = currentResVersion
	}

	return nil
}

func (w *Watch) getObject(resource string) (int, error) {
	r := strings.Split(resource, ",")

	if len(r) != 4 {
		log.Errorf("Watch: Incorrect resource format found: %s", resource)
		return 0, fmt.Errorf("Incorrect resource format")
	}

	kind := r[0]
	ns := r[1]
	apiVersion := r[2]
	name := r[3]

	obj := uns.Unstructured{}
	obj.SetKind(kind)
	obj.SetAPIVersion(apiVersion)

	err := w.client.Get(w.ctx, types.NamespacedName{Name: name, Namespace: ns}, &obj)
	if err != nil && apierrors.IsNotFound(err) {
		log.Infof("Watch: %s not found: %s/%s", kind, ns, name)
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	resVersion, err := strconv.Atoi(obj.GetResourceVersion())
	if err != nil {
		return 0, err
	}

	return resVersion, nil
}
