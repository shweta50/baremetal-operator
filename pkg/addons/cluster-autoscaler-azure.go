package addons

import (
	"path/filepath"

	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	casAzureNS     = "kube-system"
	casAzureDir    = "cluster-autoscaler/azure"
	casAzureDeploy = "cluster-autoscaler"
)

// AutoScalerAzureClient represents implementation for interacting with plain K8s cluster
type AutoScalerAzureClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newAutoScalerAzure(c client.Client, version string, params map[string]interface{}) *AutoScalerAzureClient {

	cl := &AutoScalerAzureClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//ValidateParams validates params of an addon
func (c *AutoScalerAzureClient) ValidateParams() (bool, error) {
	if _, ok := c.overrideParams["clientID"]; !ok {
		return false, addonerr.InvalidParams("clientID")
	}

	if _, ok := c.overrideParams["clientSecret"]; !ok {
		return false, addonerr.InvalidParams("clientSecret")
	}

	if _, ok := c.overrideParams["resourceGroup"]; !ok {
		return false, addonerr.InvalidParams("resourceGroup")
	}

	if _, ok := c.overrideParams["subscriptionID"]; !ok {
		return false, addonerr.InvalidParams("subscriptionID")
	}

	if _, ok := c.overrideParams["tenantID"]; !ok {
		return false, addonerr.InvalidParams("tenantID")
	}

	if _, ok := c.overrideParams["minNumWorkers"]; !ok {
		return false, addonerr.InvalidParams("minNumWorkers")
	}

	if _, ok := c.overrideParams["maxNumWorkers"]; !ok {
		return false, addonerr.InvalidParams("maxNumWorkers")
	}

	return true, nil
}

//Health checks health of the instance
func (c *AutoScalerAzureClient) Health() (bool, error) {
	deploy, err := util.GetDeployment(casAzureNS, casAzureDeploy, c.client)
	if err != nil {
		log.Errorf("Failed to get deployment: %s", err)
		return false, err
	}

	if deploy.Status.ReadyReplicas > 0 {
		return true, nil
	}

	return false, nil
}

//Upgrade upgrades an CAutoScalerAzure instance
func (c *AutoScalerAzureClient) Upgrade() error {
	return c.Install()
}

//Install installs an CAutoScalerAzure instance
func (c *AutoScalerAzureClient) Install() error {

	inputPath, outputPath, err := util.EnsureDirStructure(casAzureDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "cluster-autoscaler.yaml")
	outputFilePath := filepath.Join(outputPath, "cluster-autoscaler.yaml")

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, c.overrideParams)
	if err != nil {
		log.Errorf("Failed to write output file: %s", err)
		return err
	}

	err = util.ApplyYaml(outputFilePath, c.client)
	if err != nil {
		log.Errorf("Failed to apply yaml file: %s", err)
		return err
	}

	return nil
}

//Uninstall removes an CAutoScalerAzure instance
func (c *AutoScalerAzureClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(casAzureDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "cluster-autoscaler.yaml")
	outputFilePath := filepath.Join(outputPath, "cluster-autoscaler.yaml")

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, c.overrideParams)
	if err != nil {
		log.Errorf("Failed to write output file: %s", err)
		return err
	}

	err = util.DeleteYaml(outputFilePath, c.client)
	if err != nil {
		log.Errorf("Failed to delete yaml file: %s", err)
		return err
	}

	return nil
}
