package tests

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/addons"
)

var (
	scheme = runtime.NewScheme()
	ctx    = context.Background()
	uuid   = "8494D77F-8E26-4214-98B9-AFB50D509E60"
)

func init() {
	clientgoscheme.AddToScheme(scheme)
	agentv1.AddToScheme(scheme)
}

func getAddon(fileName string) (*agentv1.Addon, error) {
	addon := &agentv1.Addon{}

	text, err := ioutil.ReadFile("test_data/" + fileName)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(text, addon); err != nil {
		return nil, err
	}

	return addon, nil
}

func getSvc(ns, name string, c client.Client) (*corev1.Service, error) {
	svc := corev1.Service{}

	err := c.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, &svc)
	return &svc, err
}

func createAddonConfigSecret(c client.Client) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "pf9-addons",
			Name:      "addon-config",
		},
		Data: map[string][]byte{
			"dnsIP":          []byte("10.21.0.2"),
			"clientID":       []byte("QjgxMDI2RjItMjY3MS00REU3LTk1M0QtODg0NDc5QkFBM0ZCCg=="),
			"clientSecret":   []byte("MDEzQ0FGMUYtNDc1Ni00N0ZBLUJCMzItMDM2RUE4MzZFOUFFCg=="),
			"resourceGroup":  []byte("MTEwQjdGRkYtQzY0RC00MDU5LTg5RUYtNjIyM0Y5N0NBM0M5Cg=="),
			"subscriptionID": []byte("MTZDQjkyMTgtRTA3RC00NzNFLUE5MEEtOTNDNDEyMzFFRjNCCg=="),
			"tenantID":       []byte("Nzc5NEI2QkItMEI1RS00NzgxLTkzNjQtNjZCNzlEQTVEQ0ZECg=="),
		},
	}

	err := c.Create(context.Background(), secret)
	if err != nil {
		return err
	}

	return nil
}

func TestCoreDNS(t *testing.T) {
	ns := "kube-system"
	name := "kube-dns"

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	assert.Equal(t, nil, err)

	//Default coredns without additional DNS config
	addon, err := getAddon("coredns.addon")
	assert.Equal(t, nil, err)

	addonWithDNS := addon.DeepCopy()
	addonWithIP := addon.DeepCopy()

	addonWithDNS.Spec.Override.Params = append(addonWithDNS.Spec.Override.Params, agentv1.Params{
		Name:  "base64EncAdditionalDnsConfig",
		Value: "a2ZwbGMuY29tOjUzIHsKCWVycm9ycwoJY2FjaGUgMzAKCWZvcndhcmQgLiAxMC4yNDYuNi4xCn0K",
	})

	//If DNS server is specified no need to get it from addon-config
	addonWithIP.Spec.Override.Params = append(addon.Spec.Override.Params, agentv1.Params{
		Name:  "dnsServer",
		Value: "10.21.0.1",
	})

	//Addon for which dnsServer is specified
	err = w.SyncEvent(addonWithIP, "install")
	assert.Equal(t, nil, err)

	svc, err := getSvc(ns, name, client)
	assert.Equal(t, nil, err)
	assert.Equal(t, "10.21.0.1", svc.Spec.ClusterIP)

	err = w.SyncEvent(addonWithIP, "uninstall")
	assert.Equal(t, nil, err)

	//Create addon-config secret
	err = createAddonConfigSecret(client)
	assert.Equal(t, nil, err)

	//Addon for which dnsServer is not specified, should get it from addon-config
	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	svc, err = getSvc(ns, name, client)
	assert.Equal(t, nil, err)
	assert.Equal(t, "10.21.0.2", svc.Spec.ClusterIP)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)

	//Addon for which additional DNS Config is specified
	err = w.SyncEvent(addonWithDNS, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithDNS, "uninstall")
	assert.Equal(t, nil, err)
}

func TestDashboard(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := getAddon("dashboard.addon")
	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	//Expect this to failed because secret is not present
	err = w.SyncEvent(addon, "install")
	assert.NotEqual(t, nil, err)

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

	err = client.Create(ctx, &dashSecret)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)
}

func TestMetricsServer(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := getAddon("metric-server.addon")

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)
}

func TestMetallb(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := getAddon("metallb.addon")
	addonMultiRange := addon.DeepCopy()

	addon.Spec.Override.Params = append(addon.Spec.Override.Params, agentv1.Params{
		Name:  "MetallbIpRange",
		Value: "10.0.0.21-10.0.0.25",
	})

	addonMultiRange.Spec.Override.Params = append(addonMultiRange.Spec.Override.Params, agentv1.Params{
		Name:  "MetallbIpRange",
		Value: "10.0.0.21-10.0.0.25, 10.0.0.30-10.0.0.32, 10.0.0.40-10.0.0.42",
	})

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonMultiRange, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonMultiRange, "uninstall")
	assert.Equal(t, nil, err)

}

func TestAWSAutoScaler(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := getAddon("cas-aws.addon")

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)
}

func TestAzureAutoScaler(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := getAddon("cas-azure.addon")
	addonWithoutParams := addon.DeepCopy()

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)

	//Create addon-config secret
	err = createAddonConfigSecret(client)
	assert.Equal(t, nil, err)

	addonWithoutParams.Spec.Override.Params = []agentv1.Params{
		agentv1.Params{
			Name:  "minNumWorkers",
			Value: "1",
		},
		agentv1.Params{
			Name:  "maxNumWorkers",
			Value: "10",
		},
	}

	err = w.SyncEvent(addonWithoutParams, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithoutParams, "uninstall")
	assert.Equal(t, nil, err)

}
