package addons

import (
	"encoding/base64"
	"fmt"
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

//overrideRegistry checks if we need to override container registry values
func (c *AutoScalerAzureClient) overrideRegistry() {
	c.overrideParams[templateK8sRegistry] = util.GetRegistry(envVarK8sRegistry, defaultK8sRegistry)
	log.Infof("Using container registry: %s", c.overrideParams[templateK8sRegistry])
}

//ValidateParams validates params of an addon
func (c *AutoScalerAzureClient) ValidateParams() (bool, error) {
	params := []string{"minNumWorkers", "maxNumWorkers"}
	azureCredsParams := []string{"clientID", "clientSecret", "resourceGroup", "subscriptionID", "tenantID"}

	for _, p := range params {
		if _, ok := c.overrideParams[p]; !ok {
			return false, addonerr.InvalidParams(p)
		}
	}

	//If all params specified no need to get them from addon-config
	if len(c.overrideParams) == 7 {
		for _, p := range azureCredsParams {
			if _, ok := c.overrideParams[p]; !ok {
				return false, addonerr.InvalidParams(p)
			}
		}

		return true, nil
	}

	sec, err := util.GetSecret(addonsNS, addonsConfigSecret, c.client)
	if err != nil {
		log.Errorf("Failed to get addon-config secret: %s", err)
		return false, err
	}

	if sec == nil {
		log.Error("addon-config secret not found")
		return false, fmt.Errorf("addon-config secret not found")
	}

	for _, p := range azureCredsParams {
		val, ok := sec.Data[p]
		if !ok {
			log.Errorf("%s not found in addon-config", p)
			return false, fmt.Errorf("%s not found in addon-config", p)
		}

		c.overrideParams[p] = base64.StdEncoding.EncodeToString(val)
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

	if deploy == nil {
		return false, nil
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

	b, err := util.CheckClusterUpgrading(c.client)
	if err != nil {
		return err
	}

	if b {
		return fmt.Errorf("Cluster is upgrading ignoring request")
	}

	inputPath, outputPath, err := util.EnsureDirStructure(casAzureDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "cluster-autoscaler.yaml")
	outputFilePath := filepath.Join(outputPath, "cluster-autoscaler.yaml")

	c.overrideRegistry()

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, "cluster-autoscaler.yaml", c.overrideParams)
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

	b, err := util.CheckClusterUpgrading(c.client)
	if err != nil {
		return err
	}

	if b {
		return fmt.Errorf("Cluster is upgrading ignoring request")
	}

	inputPath, outputPath, err := util.EnsureDirStructure(casAzureDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "cluster-autoscaler.yaml")
	outputFilePath := filepath.Join(outputPath, "cluster-autoscaler.yaml")

	c.overrideRegistry()

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, "cluster-autoscaler.yaml", c.overrideParams)
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
