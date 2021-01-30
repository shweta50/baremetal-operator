package k8s

import (
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	casAzureNS     = "kube-system"
	casAzureDir    = "cluster-autoscaler/azure"
	casAzureDeploy = "cluster-autoscaler"
)

// CAutoScalerAzureClient represents implementation for interacting with plain K8s cluster
type CAutoScalerAzureClient struct {
	c              client.Client
	overrideParams map[string]interface{}
	version        string
}

func getCAutoScalerAzure(c client.Client, version string, params map[string]interface{}) *CAutoScalerAzureClient {

	cl := &CAutoScalerAzureClient{
		c:              c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//Health checks health of the instance
func (c *CAutoScalerAzureClient) Health() (bool, error) {
	deploy, err := util.GetDeployment(casAzureNS, casAzureDeploy, c.c)
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
func (c *CAutoScalerAzureClient) Upgrade() error {
	return c.Install()
}

//Install installs an CAutoScalerAzure instance
func (c *CAutoScalerAzureClient) Install() error {

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

	err = util.ApplyYaml(outputFilePath, c.c)
	if err != nil {
		log.Errorf("Failed to apply yaml file: %s", err)
		return err
	}

	return nil
}

//Uninstall removes an CAutoScalerAzure instance
func (c *CAutoScalerAzureClient) Uninstall() error {

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

	err = util.DeleteYaml(outputFilePath, c.c)
	if err != nil {
		log.Errorf("Failed to delete yaml file: %s", err)
		return err
	}

	return nil
}
