package watch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/addons"
	"github.com/platform9/pf9-addon-operator/pkg/watch"
	util "github.com/platform9/pf9-addon-operator/tests/util"
	sunpikev1alpha2 "github.com/platform9/pf9-qbert/sunpike/apiserver/pkg/apis/sunpike/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	scheme    = runtime.NewScheme()
	ctx       = context.Background()
	watchUUID = "8494D77F-8E26-4214-98B9-AFB50D509E60"
)

func init() {
	clientgoscheme.AddToScheme(scheme)
	agentv1.AddToScheme(scheme)
}

func TestCoreDNSWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)
	ch := make(chan bool)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite coredns addon
	addon, err := util.GetAddon("coredns.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-coredns"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID
	addon.Spec.Override.Params = append(addon.Spec.Override.Params, agentv1.Params{
		Name:  "dnsServer",
		Value: "10.21.0.1",
	})
	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Status.ObservedGeneration = 10

	// Create coredns addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of coredns resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update coredns deployment by adding a label
	d, err := util.GetDeployment(ctx, "kube-system", "coredns", cl)
	assert.Equal(t, nil, err)

	d.Labels["abc"] = "xyz"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
		ch <- true
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of coredns resources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)

	// wait till the above running watch exits
	// Simulate Reconcile loop to respond to watch.Run updating Addon object
	time.Sleep(7 * time.Second)
	cl.Get(ctx, addonKey, addon)
	w.SyncEvent(addon, "install")
	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
	<-ch

	// Test if coredns configmap change is detected and rolled back
	cl.Get(ctx, addonKey, addon)
	obsGen3 := addon.Status.ObservedGeneration

	// Manually change the metallb configmap
	cm, err := util.GetConfigMap(ctx, "kube-system", "coredns", cl)
	assert.Equal(t, nil, err)

	cm.Data["Corefile"] = "xyz"

	err = util.SetConfigMap(ctx, cm, cl)
	assert.Equal(t, nil, err)

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen4 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of coredns resources and is trying to trigger Addon object
	assert.True(t, obsGen3 > obsGen4)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}
func TestDashboardWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite dashboard addon
	addon, err := util.GetAddon("dashboard.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-dashboard"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID

	dashSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-dashboard-certs",
			Namespace: "kubernetes-dashboard",
		},
		Data: map[string][]byte{
			"dashboard.key": []byte("key"),
			"dashboard.crt": []byte("crt"),
		},
	}

	err = cl.Create(ctx, &dashSecret)
	assert.Equal(t, nil, err)

	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Status.ObservedGeneration = 10

	// Create dashboard addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of dashboard resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update dashboard deployment by adding a label
	d, err := util.GetDeployment(ctx, "kubernetes-dashboard", "kubernetes-dashboard", cl)
	assert.Equal(t, nil, err)

	d.Labels["mkn"] = "bhv"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of dashboard resources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}

func TestLuigiWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite luigi addon
	addon, err := util.GetAddon("luigi.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-luigi"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID
	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Status.ObservedGeneration = 10

	// Create luigi addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of luigi resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update luigi deployment by adding a label
	d, err := util.GetDeployment(ctx, "luigi-system", "luigi-controller-manager", cl)
	assert.Equal(t, nil, err)

	d.Labels["xyz"] = "abc"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of luigi resources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}

func TestProfileAgentWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite pf9 profile agent addon
	addon, err := util.GetAddon("pf9-profile-agent.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-pf9-profile-agent"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID
	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Status.ObservedGeneration = 10

	// Create pf9 profile agent addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of profile agent resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update profile agent deployment by adding a label
	d, err := util.GetDeployment(ctx, "platform9-system", "pf9-profile-agent", cl)
	assert.Equal(t, nil, err)

	d.Labels["xyz"] = "abc"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of profile agentresources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}

func TestKubevirtWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite kubevirt addon
	addon, err := util.GetAddon("kubevirt.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-kubevirt"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID
	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Status.ObservedGeneration = 10

	// Create kubevirt addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of kubevirt resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update kubevirt deployment by adding a label
	d, err := util.GetDeployment(ctx, "kubevirt", "virt-operator", cl)
	assert.Equal(t, nil, err)

	d.Labels["xyz"] = "abc"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of kubevirt resources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}

func TestMetricServerWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite metrics server addon
	addon, err := util.GetAddon("metric-server.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-metric-server"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID
	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Status.ObservedGeneration = 10

	// Create metrics server addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of metrics server resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update metrics server deployment by adding a label
	d, err := util.GetDeployment(ctx, "kube-system", "metrics-server-v0.5.0", cl)
	assert.Equal(t, nil, err)

	d.Labels["xyz"] = "abc"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of metrics server resources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}

func TestAzureCASWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite azure cluster addon
	addon, err := util.GetAddon("cas-azure.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-cas-azure"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID
	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Status.ObservedGeneration = 10

	// Create azure cluster addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of azure cluster resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update azure cluster deployment by adding a label
	d, err := util.GetDeployment(ctx, "kube-system", "cluster-autoscaler", cl)
	assert.Equal(t, nil, err)

	d.Labels["xyz"] = "abc"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of azure cluster resources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}

func TestMetallbWatch(t *testing.T) {

	cl := fake.NewFakeClientWithScheme(scheme)
	ch := make(chan bool)

	w, err := addons.New(cl)
	assert.Equal(t, nil, err)

	wtc, err := watch.New(ctx, cl)
	assert.Equal(t, nil, err)

	//Initialite metallb addon
	addon, err := util.GetAddon("metallb.addon")
	assert.Equal(t, nil, err)

	addon.Name = watchUUID + "-metallb"
	addon.Namespace = "pf9-addons"
	addon.Spec.ClusterID = watchUUID
	addon.Status.Phase = sunpikev1alpha2.AddonPhase("Installed")
	addon.Spec.Override.Params = append(addon.Spec.Override.Params, agentv1.Params{
		Name:  "MetallbIpRange",
		Value: "10.0.0.21-10.0.0.25",
	})
	addon.Status.ObservedGeneration = 10

	// Create metallb addon object
	err = cl.Create(ctx, addon)
	assert.Equal(t, nil, err)

	// Install it
	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	addonKey := client.ObjectKey{
		Namespace: "pf9-addons",
		Name:      addon.Name,
	}

	go func() {
		// Simulate Reconcile loop to respond to watch.Run updating Addon object
		time.Sleep(7 * time.Second)
		cl.Get(ctx, addonKey, addon)
		w.SyncEvent(addon, "install")
		addon.Status.ObservedGeneration = 10
		cl.Status().Update(ctx, addon)
	}()

	// watch.Run will warm up it cache by running updating Addon object and
	// reading resource versions of metallb resources
	err = wtc.Run()
	assert.Equal(t, nil, err)

	// Update metallb deployment by adding a label
	d, err := util.GetDeployment(ctx, "metallb-system", "controller", cl)
	assert.Equal(t, nil, err)

	d.Labels["xyz"] = "abc"

	err = util.SetDeployment(ctx, d, cl)
	assert.Equal(t, nil, err)

	// Save the observed generation before running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen1 := addon.Status.ObservedGeneration

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
		ch <- true
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen2 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of metallb resources and is trying to trigger Addon object
	assert.True(t, obsGen1 > obsGen2)
	// wait till the above running watch exits
	// Simulate Reconcile loop to respond to watch.Run updating Addon object
	time.Sleep(7 * time.Second)
	cl.Get(ctx, addonKey, addon)
	w.SyncEvent(addon, "install")
	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
	<-ch

	// Test if metallb configmap change is detected and rolled back
	cl.Get(ctx, addonKey, addon)
	obsGen3 := addon.Status.ObservedGeneration

	// Manually change the metallb configmap
	cm, err := util.GetConfigMap(ctx, "metallb-system", "config", cl)
	assert.Equal(t, nil, err)

	cm.Data["config"] = "xyz"

	err = util.SetConfigMap(ctx, cm, cl)
	assert.Equal(t, nil, err)

	go func() {
		err = wtc.Run()
		assert.Equal(t, nil, err)
	}()

	time.Sleep(4 * time.Second)

	// Save the observed generation after running watch.Run
	cl.Get(ctx, addonKey, addon)
	obsGen4 := addon.Status.ObservedGeneration

	// obsGen1 should be greater than obsGen2, confirms that watch has detected a change
	// in one of metallb resources and is trying to trigger Addon object
	assert.True(t, obsGen3 > obsGen4)

	addon.Status.ObservedGeneration = 10
	cl.Status().Update(ctx, addon)
}
