package addonstest

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
	util "github.com/platform9/pf9-addon-operator/tests/util"
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

func TestCoreDNS(t *testing.T) {
	ns := "kube-system"
	name := "kube-dns"

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	assert.Equal(t, nil, err)

	//Default coredns without additional DNS config
	addon, err := util.GetAddon("coredns.addon")
	assert.Equal(t, nil, err)

	addonWithDNS := addon.DeepCopy()
	addonWithIP := addon.DeepCopy()
	addonWithDNSEnc := addon.DeepCopy()

	addonWithDNS.Spec.Override.Params = append(addonWithDNS.Spec.Override.Params, agentv1.Params{
		Name:  "base64EncAdditionalDnsConfig",
		Value: "a2ZwbGMuY29tOjUzIHsKCWVycm9ycwoJY2FjaGUgMzAKCWZvcndhcmQgLiAxMC4yNDYuNi4xCn0K",
	})

	//If DNS server is specified no need to get it from addon-config
	addonWithIP.Spec.Override.Params = append(addon.Spec.Override.Params, agentv1.Params{
		Name:  "dnsServer",
		Value: "10.21.0.1",
	})

	addonWithDNSEnc.Spec.Override.Params = append(addonWithDNSEnc.Spec.Override.Params, agentv1.Params{
		Name:  "base64EncEntireDnsConfig",
		Value: "Ljo1MzAwIHsKICAgIGVycm9ycwogICAgaGVhbHRoIHsKICAgICAgICBsYW1lZHVjayA1cwogICAgfQogICAgcmVhZHkKICAgIGt1YmVybmV0ZXMgY2x1c3Rlci5sb2NhbCBpbi1hZGRyLmFycGEgaXA2LmFycGEgewogICAgICAgIHBvZHMgaW5zZWN1cmUKICAgICAgICBmYWxsdGhyb3VnaCBpbi1hZGRyLmFycGEgaXA2LmFycGEKICAgICAgICB0dGwgMzAKICAgIH0KICAgIHByb21ldGhldXMgOjkxNTMKICAgIGZvcndhcmQgLiAvZXRjL3Jlc29sdi5jb25mIHsKICAgICAgICBtYXhfY29uY3VycmVudCAxMDAwCiAgICB9CmNhY2hlIDMwCiAgICBsb29wCiAgICByZWxvYWQKICAgIGxvYWRiYWxhbmNlCn0K",
	})

	//Addon for which dnsServer is specified
	err = w.SyncEvent(addonWithIP, "install")
	assert.Equal(t, nil, err)

	svc, err := util.GetSvc(ctx, ns, name, client)
	assert.Equal(t, nil, err)
	assert.Equal(t, "10.21.0.1", svc.Spec.ClusterIP)

	err = w.SyncEvent(addon, "upgrade")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithIP, "uninstall")
	assert.Equal(t, nil, err)

	//Create addon-config secret
	err = util.CreateAddonConfigSecret(ctx, client)
	assert.Equal(t, nil, err)

	//Addon for which dnsServer is not specified, should get it from addon-config
	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	svc, err = util.GetSvc(ctx, ns, name, client)
	assert.Equal(t, nil, err)
	assert.Equal(t, "10.21.0.2", svc.Spec.ClusterIP)

	err = w.SyncEvent(addon, "upgrade")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)

	//Addon for which additional DNS Config is specified
	err = w.SyncEvent(addonWithDNS, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "upgrade")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithDNS, "uninstall")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithDNSEnc, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "upgrade")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithDNSEnc, "uninstall")
	assert.Equal(t, nil, err)

}

func TestDashboard(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := util.GetAddon("dashboard.addon")
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

	addon, _ := util.GetAddon("metric-server.addon")

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

	addon, _ := util.GetAddon("metallb.addon")
	addonMultiRange := addon.DeepCopy()
	addonWithBase64Enc := addon.DeepCopy()

	addon.Spec.Override.Params = append(addon.Spec.Override.Params, agentv1.Params{
		Name:  "MetallbIpRange",
		Value: "10.0.0.21-10.0.0.25",
	})

	addonMultiRange.Spec.Override.Params = append(addonMultiRange.Spec.Override.Params, agentv1.Params{
		Name:  "MetallbIpRange",
		Value: "10.0.0.21-10.0.0.25, 10.0.0.30-10.0.0.32, 10.0.0.40-10.0.0.42",
	})

	addonWithBase64Enc.Spec.Override.Params = append(addonMultiRange.Spec.Override.Params, agentv1.Params{
		Name:  "base64EncMetallbConfig",
		Value: "YWRkcmVzcy1wb29sczoKLSBuYW1lOiBkZWZhdWx0CiAgcHJvdG9jb2w6IGxheWVyMgogIGFkZHJlc3NlczoKICAgLSAxOTIuMTY4LjUuMC0xOTIuMTY4LjYuMAotIG5hbWU6IHBvb2wKICBwcm90b2NvbDogbGF5ZXIyCiAgYWRkcmVzc2VzOgogICAtIDE5Mi4xNjguNy4wLTE5Mi4xNjguOC4wCg==",
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

	err = w.SyncEvent(addonWithBase64Enc, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithBase64Enc, "uninstall")
	assert.Equal(t, nil, err)

}

func TestAWSAutoScaler(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := util.GetAddon("cas-aws.addon")

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

	addon, _ := util.GetAddon("cas-azure.addon")
	addonWithoutParams := addon.DeepCopy()

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)

	//Create addon-config secret
	err = util.CreateAddonConfigSecret(ctx, client)
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

func TestKubevirt(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := util.GetAddon("kubevirt.addon")

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)
}

func TestMonitoring(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := util.GetAddon("monitoring.addon")
	addonWithoutParams := addon.DeepCopy()

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)

	//Create addon-config secret
	err = util.CreateAddonConfigSecret(ctx, client)
	assert.Equal(t, nil, err)

	addonWithoutParams.Spec.Override.Params = []agentv1.Params{
		agentv1.Params{
			Name:  "StorageClassName",
			Value: "default",
		},
		agentv1.Params{
			Name:  "pvcSize",
			Value: "1Gi",
		},
		agentv1.Params{
			Name:  "retentionTime",
			Value: "7d",
		},
	}

	err = w.SyncEvent(addonWithoutParams, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addonWithoutParams, "uninstall")
	assert.Equal(t, nil, err)

}

func TestLuigi(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := util.GetAddon("luigi.addon")

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)
}

func TestProfileAgent(t *testing.T) {

	client := fake.NewFakeClientWithScheme(scheme)

	w, err := addons.New(client)
	if err != nil {
		t.Fatal(err.Error())
	}

	addon, _ := util.GetAddon("pf9-profile-agent.addon")

	err = client.Create(ctx, addon)
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "install")
	assert.Equal(t, nil, err)

	err = w.SyncEvent(addon, "uninstall")
	assert.Equal(t, nil, err)
}
