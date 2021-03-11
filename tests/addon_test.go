package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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

func createCoreDNS() *agentv1.Addon {
	addon := agentv1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid + "-coredns",
			Namespace: "pf9-addons",
		},
		Spec: agentv1.AddonSpec{
			ClusterID: uuid,
			Version:   "1.7.0",
			Type:      "coredns",
		},
	}

	addon.Spec.Override.Params = []agentv1.Params{
		agentv1.Params{
			Name:  "dnsMemoryLimit",
			Value: "170Mi",
		},
		agentv1.Params{
			Name:  "dnsDomain",
			Value: "cluster.local",
		},
		agentv1.Params{
			Name:  "dnsServer",
			Value: "10.21.0.1",
		},
	}

	return &addon
}

func createMetallb() *agentv1.Addon {
	addon := agentv1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid + "-metallb",
			Namespace: "pf9-addons",
		},
		Spec: agentv1.AddonSpec{
			ClusterID: uuid,
			Version:   "0.9.5",
			Type:      "metallb",
		},
	}

	return &addon
}

func createDashboard() *agentv1.Addon {
	addon := agentv1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid + "-dashboard",
			Namespace: "pf9-addons",
		},
		Spec: agentv1.AddonSpec{
			ClusterID: uuid,
			Version:   "2.0.3",
			Type:      "kubernetes-dashboard",
		},
	}

	return &addon
}

func createMetricsServer() *agentv1.Addon {
	addon := agentv1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid + "-metrics-server",
			Namespace: "pf9-addons",
		},
		Spec: agentv1.AddonSpec{
			ClusterID: uuid,
			Version:   "0.3.6",
			Type:      "metrics-server",
		},
	}

	return &addon
}

func createAWSAutoScaler() *agentv1.Addon {
	addon := agentv1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid + "-aws",
			Namespace: "pf9-addons",
		},
		Spec: agentv1.AddonSpec{
			ClusterID: uuid,
			Version:   "1.14.7",
			Type:      "cluster-auto-scaler-aws",
		},
	}

	addon.Spec.Override.Params = []agentv1.Params{
		agentv1.Params{
			Name:  "clusterUUID",
			Value: "uuid",
		},
		agentv1.Params{
			Name:  "clusterRegion",
			Value: "region",
		},
		agentv1.Params{
			Name:  "cpuRequest",
			Value: "100m",
		},
		agentv1.Params{
			Name:  "cpuLimit",
			Value: "200m",
		},
		agentv1.Params{
			Name:  "memRequest",
			Value: "300Mi",
		},
		agentv1.Params{
			Name:  "memLimit",
			Value: "600Mi",
		},
	}

	return &addon
}

func createAzureAutoScaler() *agentv1.Addon {

	addon := agentv1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid + "-azure",
			Namespace: "pf9-addons",
		},
		Spec: agentv1.AddonSpec{
			ClusterID: uuid,
			Version:   "1.13.8",
			Type:      "cluster-auto-scaler-azure",
		},
	}

	addon.Spec.Override.Params = []agentv1.Params{
		agentv1.Params{
			Name:  "clientID",
			Value: "uuid",
		},
		agentv1.Params{
			Name:  "clientSecret",
			Value: "secret",
		},
		agentv1.Params{
			Name:  "resourceGroup",
			Value: "resourcegroup",
		},
		agentv1.Params{
			Name:  "subscriptionID",
			Value: "subID",
		},
		agentv1.Params{
			Name:  "tenantID",
			Value: "tenID",
		},
		agentv1.Params{
			Name:  "minNumWorkers",
			Value: "1",
		},
		agentv1.Params{
			Name:  "maxNumWorkers",
			Value: "10",
		},
	}

	return &addon
}

func TestCoreDNS(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon := createCoreDNS()
	addonWithDNS := addon.DeepCopy()

	addon.Spec.Override.Params = append(addon.Spec.Override.Params, agentv1.Params{
		Name:  "enableAdditionalDnsConfig",
		Value: "false",
	})

	addonWithDNS.Spec.Override.Params = append(addonWithDNS.Spec.Override.Params, agentv1.Params{
		Name:  "enableAdditionalDnsConfig",
		Value: "true",
	})

	addonWithDNS.Spec.Override.Params = append(addonWithDNS.Spec.Override.Params, agentv1.Params{
		Name:  "base64EncAdditionalDnsConfig",
		Value: "ICAgICAga2ZwbGMuY29tOjUzIHsKICAgICAgZXJyb3JzCiAgICAgIGNhY2hlIDMwCiAgICAgIGZvcndhcmQgLiAxMC4yNDYuNi4xCiAgICAgIH0K",
	})

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)

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

	addon := createDashboard()

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

	addon := createMetricsServer()

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

	addon := createMetallb()
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

	addon := createAWSAutoScaler()

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

	addon := createAzureAutoScaler()

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)
}
