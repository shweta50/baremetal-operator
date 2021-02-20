package k8s

import (
	"path/filepath"

	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	casAWSNS     = "kube-system"
	casAWSDir    = "cluster-autoscaler/aws"
	casAWSDeploy = "cluster-autoscaler"
)

// CAutoScalerAwsClient represents implementation for interacting with plain K8s cluster
type CAutoScalerAwsClient struct {
	c              client.Client
	overrideParams map[string]interface{}
	version        string
}

func getCAutoScalerAws(c client.Client, version string, params map[string]interface{}) *CAutoScalerAwsClient {

	cl := &CAutoScalerAwsClient{
		c:              c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//ValidateParams validates params of an addon
func (c *CAutoScalerAwsClient) ValidateParams() (bool, error) {
	if _, ok := c.overrideParams["clusterUUID"]; !ok {
		return false, addonerr.InvalidParams("clusterUUID")
	}

	if _, ok := c.overrideParams["clusterRegion"]; !ok {
		return false, addonerr.InvalidParams("clusterRegion")
	}

	return true, nil
}

//Health checks health of the instance
func (c *CAutoScalerAwsClient) Health() (bool, error) {
	deploy, err := util.GetDeployment(casAWSNS, casAWSDeploy, c.c)
	if err != nil {
		log.Errorf("Failed to get deployment: %s", err)
		return false, err
	}

	if deploy.Status.ReadyReplicas > 0 {
		return true, nil
	}

	return false, nil
}

//Upgrade upgrades an CAutoScalerAws instance
func (c *CAutoScalerAwsClient) Upgrade() error {
	return c.Install()
}

//Install installs an CAutoScalerAws instance
func (c *CAutoScalerAwsClient) Install() error {

	inputPath, outputPath, err := util.EnsureDirStructure(casAWSDir, c.version)
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

//Uninstall removes an CAutoScalerAws instance
func (c *CAutoScalerAwsClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(casAWSDir, c.version)
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
