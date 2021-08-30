package healthcheck

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/addons"
	sunpikev1alpha2 "github.com/platform9/pf9-qbert/sunpike/apiserver/pkg/apis/sunpike/v1alpha2"
)

var (
	ctx           = context.Background()
	localScheme   = runtime.NewScheme()
	sunpikeScheme = runtime.NewScheme()
	//local client represents PMK cluster
	localClient client.Client
	//sunpike client represents sunpike apiserver
	sunpikeClient client.Client
)

func init() {
	clientgoscheme.AddToScheme(localScheme)
	agentv1.AddToScheme(localScheme)
	sunpikev1alpha2.AddToScheme(sunpikeScheme)
	sunpikeScheme.SetVersionPriority(sunpikev1alpha2.SchemeGroupVersion)

	localClient = fake.NewFakeClientWithScheme(localScheme)
	sunpikeClient = fake.NewFakeClientWithScheme(sunpikeScheme)
}

func newMux() *mux.Router {
	return mux.NewRouter()
}

func getFakeSunpikeKubeCfg(url string) (*rest.Config, error) {
	data, err := ioutil.ReadFile("../fake_kubeconfig.template")
	if err != nil {
		return nil, err
	}

	buf := strings.Replace(string(data), "__DU_QBERT_FQDN__", url, 1)
	buf = strings.Replace(buf, "__PROJECT_ID__", "projectid", 1)

	kubeCfgPath := "fake.cfg"

	err = ioutil.WriteFile(kubeCfgPath, []byte(buf), 0600)
	if err != nil {
		return nil, err
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	if err != nil {
		return nil, err
	}

	return cfg, err
}

func getFakeClusterAddons(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cn := vars["cluster"]
	clsAddon := &sunpikev1alpha2.ClusterAddon{}

	key := client.ObjectKey{
		Namespace: "",
		Name:      cn,
	}

	if err := sunpikeClient.Get(ctx, key, clsAddon); err != nil {
		fmt.Printf("Failed to get addons: %s", err.Error())
		return
	}

	b, _ := json.Marshal(clsAddon)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}

func postFakeClusterAddons(w http.ResponseWriter, r *http.Request) {
	clsAddon := &sunpikev1alpha2.ClusterAddon{}

	json.NewDecoder(r.Body).Decode(clsAddon)

	if err := sunpikeClient.Create(ctx, clsAddon); err != nil {
		fmt.Printf("Failed to create addons: %s", err.Error())
		return
	}

	b, _ := json.Marshal(clsAddon)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}

func putFakeClusterAddons(w http.ResponseWriter, r *http.Request) {
	clsAddonUpdate := &sunpikev1alpha2.ClusterAddon{}
	clsAddonGet := &sunpikev1alpha2.ClusterAddon{}

	json.NewDecoder(r.Body).Decode(clsAddonUpdate)

	key := client.ObjectKey{
		Namespace: "default",
		Name:      clsAddonUpdate.Name,
	}

	if err := sunpikeClient.Get(ctx, key, clsAddonGet); err != nil {
		fmt.Printf("Failed to get addons: %s in putFakeClusterAddons", err.Error())
		return
	}

	clsAddonGet.Status = clsAddonUpdate.Status

	if err := sunpikeClient.Update(ctx, clsAddonGet); err != nil {
		fmt.Printf("Failed to update addons: %s", err.Error())
		return
	}

	b, _ := json.Marshal(clsAddonGet)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}

func listFakeClusterAddons(w http.ResponseWriter, r *http.Request) {
	clsAddonList := &sunpikev1alpha2.ClusterAddonList{}

	if err := sunpikeClient.List(ctx, clsAddonList); err != nil {
		fmt.Printf("Failed to list addons: %s", err.Error())
		return
	}

	b, _ := json.Marshal(clsAddonList)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}

func addAPI(m *mux.Router) {
	m.HandleFunc("/qbert/v4/{projectid}/sunpike/apis/sunpike.platform9.com/v1alpha2/namespaces/default/clusteraddons/{cluster}", getFakeClusterAddons).Methods("GET")
	m.HandleFunc("/qbert/v4/{projectid}/sunpike/apis/sunpike.platform9.com/v1alpha2/namespaces/default/clusteraddons", postFakeClusterAddons).Methods("POST")
	m.HandleFunc("/qbert/v4/{projectid}/sunpike/apis/sunpike.platform9.com/v1alpha2/namespaces/default/clusteraddons/{cluster}", putFakeClusterAddons).Methods("PUT")
	m.HandleFunc("/qbert/v4/{projectid}/sunpike/apis/sunpike.platform9.com/v1alpha2/namespaces/default/clusteraddons/{cluster}", putFakeClusterAddons).Methods("PATCH")
	m.HandleFunc("/qbert/v4/{projectid}/sunpike/apis/sunpike.platform9.com/v1alpha2/namespaces/default/clusteraddons", listFakeClusterAddons).Methods("GET")
}

func createClusterAddonFromFile(fileName string) error {

	clsAddon := &sunpikev1alpha2.ClusterAddon{}

	text, err := ioutil.ReadFile("../test_data/" + fileName)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(text, clsAddon); err != nil {
		return err
	}

	createClusterAddon(clsAddon)
	return nil
}

func createClusterAddon(clsAddon *sunpikev1alpha2.ClusterAddon) {
	if err := sunpikeClient.Create(ctx, clsAddon); err != nil {
		fmt.Printf("Failed to create ClusterAddon object: %s", err.Error())
	}
}

func updateClusterAddon(clsAddon *sunpikev1alpha2.ClusterAddon) error {

	clsAddonGet := &sunpikev1alpha2.ClusterAddon{}

	key := client.ObjectKey{
		Namespace: "default",
		Name:      clsAddon.Name,
	}

	if err := sunpikeClient.Get(ctx, key, clsAddonGet); err != nil {
		fmt.Printf("Failed to get addons: %s", err.Error())
		return err
	}

	clsAddonGet.DeletionTimestamp = clsAddon.DeletionTimestamp
	clsAddonGet.Spec = clsAddon.Spec

	if err := sunpikeClient.Update(ctx, clsAddonGet); err != nil {
		fmt.Printf("Failed to update ClusterAddon object: %s", err.Error())
	}

	return nil
}

func updateAddon(addon *agentv1.Addon) error {

	addonGet := &agentv1.Addon{}

	key := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	if err := localClient.Get(ctx, key, addonGet); err != nil {
		fmt.Printf("Failed to get addons: %s", err.Error())
		return err
	}

	addonGet.Status = addon.Status

	if err := localClient.Update(ctx, addonGet); err != nil {
		fmt.Printf("Failed to update Addon object: %s", err.Error())
	}

	return nil
}

func TestHealthCheck(t *testing.T) {

	w, err := addons.New(localClient)
	assert.Equal(t, nil, err)

	m := newMux()
	addAPI(m)
	ts := httptest.NewServer(m)
	defer ts.Close()

	kubeCfg, err := getFakeSunpikeKubeCfg(ts.URL)
	assert.Equal(t, nil, err)

	//------------------------------ Test 1 --------------------------------------
	//Newly created ClusterAddon objects during boostrap should be reflected as local Addon objects
	//Create 6 local ClusterAddon objects
	clsAddonManifests := []string{"coredns.clusteraddon", "dashboard.clusteraddon", "metallb.clusteraddon", "metric-server.clusteraddon", "cas-aws.clusteraddon", "cas-azure.clusteraddon"}
	for _, f := range clsAddonManifests {
		createClusterAddonFromFile(f)
	}

	//Sync between sunpike and local cluster
	err = w.SyncClusterAddons("clusterid", "projectid", kubeCfg)
	assert.Equal(t, nil, err)

	//Verify if 6 Addon objects have been created
	addonList := &agentv1.AddonList{}
	err = localClient.List(ctx, addonList)
	assert.Equal(t, nil, err)
	assert.Equal(t, 6, len(addonList.Items))

	//------------------------------ Test 2 --------------------------------------
	//New ClusterAddon object should be reflected as local Addon object
	//Create object on sunpike
	clsAddon := sunpikev1alpha2.ClusterAddon{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "uuid-coredns",
			Namespace:  "default",
			Finalizers: []string{"pf9.io/addons"},
		},
		Spec: sunpikev1alpha2.ClusterAddonSpec{
			ClusterID: "uuid",
			Version:   "1.7.0",
			Type:      "coredns",
		},
	}
	createClusterAddon(&clsAddon)

	//Sync between sunpike and local cluster
	err = w.SyncClusterAddons("clusterid", "projectid", kubeCfg)
	assert.Equal(t, nil, err)

	//Check if local Addon object has been created
	addon := &agentv1.Addon{}
	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      "uuid-coredns",
	}

	clsAddonKey := client.ObjectKey{
		Namespace: "default",
		Name:      "uuid-coredns",
	}

	err = localClient.Get(ctx, addonKey, addon)
	assert.Equal(t, nil, err)
	assert.Equal(t, "uuid-coredns", addon.Name)

	//------------------------------ Test 3 --------------------------------------
	//Change spec of ClusterAddon object, should reflect in corresp local Addon
	clsAddon.Spec.Version = "1.9.0"
	updateClusterAddon(&clsAddon)

	//Sync between sunpike and local cluster
	err = w.SyncClusterAddons("clusterid", "projectid", kubeCfg)
	assert.Equal(t, nil, err)

	//Check if local addon object has been updated
	err = localClient.Get(ctx, addonKey, addon)
	assert.Equal(t, nil, err)
	assert.Equal(t, "1.9.0", addon.Spec.Version)

	//------------------------------ Test 4 --------------------------------------
	//Set status of local Addon object, should be reflected in status of ClusterAddon object
	addon.Status.Phase = "Installed"
	updateAddon(addon)

	//Sync between sunpike and local cluster
	err = w.SyncClusterAddons("clusterid", "projectid", kubeCfg)
	assert.Equal(t, nil, err)

	//Check if cluster addon status has been updated
	err = sunpikeClient.Get(ctx, clsAddonKey, &clsAddon)
	assert.Equal(t, nil, err)
	assert.Equal(t, sunpikev1alpha2.AddonPhase("Installed"), clsAddon.Status.Phase)

	//------------------------------ Test 5 --------------------------------------
	//Delete cluster addon object and check if local Addon object gets deleted
	now := metav1.Now()
	clsAddon.DeletionTimestamp = &now
	updateClusterAddon(&clsAddon)

	//Sync between sunpike and local cluster
	err = w.SyncClusterAddons("clusterid", "projectid", kubeCfg)
	assert.Equal(t, nil, err)

	//Check if local addon object has been deleted
	err = localClient.Get(ctx, addonKey, addon)
	assert.NotEqual(t, nil, err)

	err = sunpikeClient.Get(ctx, clsAddonKey, &clsAddon)
	assert.Equal(t, nil, err)
	assert.Equal(t, sunpikev1alpha2.AddonPhase("Uninstalling"), clsAddon.Status.Phase)

	//Sync again between sunpike and local cluster
	err = w.SyncClusterAddons("clusterid", "projectid", kubeCfg)
	assert.Equal(t, nil, err)

	//Check if ClusterAddon object has been updated to Uninstalled
	err = sunpikeClient.Get(ctx, clsAddonKey, &clsAddon)
	assert.Equal(t, nil, err)
	assert.Equal(t, sunpikev1alpha2.AddonPhase("Uninstalled"), clsAddon.Status.Phase)
}
