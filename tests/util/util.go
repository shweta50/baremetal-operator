package util

import (
	"context"
	"encoding/json"
	"io/ioutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// GetAddon gets an addon object based on a template json
func GetAddon(fileName string) (*agentv1.Addon, error) {
	addon := &agentv1.Addon{}

	text, err := ioutil.ReadFile("../test_data/" + fileName)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(text, addon); err != nil {
		return nil, err
	}

	return addon, nil
}

// GetSvc gets a k8s service
func GetSvc(ctx context.Context, ns, name string, c client.Client) (*corev1.Service, error) {
	svc := corev1.Service{}

	err := c.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, &svc)
	return &svc, err
}

// CreateAddonConfigSecret creates a secret required by addon operator
func CreateAddonConfigSecret(ctx context.Context, c client.Client) error {
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

	err := c.Create(ctx, secret)
	if err != nil {
		return err
	}

	return nil
}

// GetDeployment gets a deployment
func GetDeployment(ctx context.Context, ns, name string, c client.Client) (*appsv1.Deployment, error) {
	d := &appsv1.Deployment{}

	err := c.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, d)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return d, nil
}

// SetDeployment sets a deployment
func SetDeployment(ctx context.Context, d *appsv1.Deployment, c client.Client) error {
	return c.Update(ctx, d)
}

// GetConfigMap gets a cfgmap
func GetConfigMap(ctx context.Context, ns, name string, c client.Client) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}

	err := c.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return cm, nil
}

//SetConfigMap updates a cfgmap
func SetConfigMap(ctx context.Context, cm *corev1.ConfigMap, c client.Client) error {
	err := c.Update(ctx, cm)
	if err != nil {
		return err
	}

	return nil
}
