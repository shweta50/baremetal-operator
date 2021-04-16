package addons

import (
	"fmt"
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

// AutoScalerAwsClient represents implementation for interacting with plain K8s cluster
type AutoScalerAwsClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newAutoScalerAws(c client.Client, version string, params map[string]interface{}) *AutoScalerAwsClient {

	cl := &AutoScalerAwsClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//overrideRegistry checks if we need to override container registry values
func (c *AutoScalerAwsClient) overrideRegistry() {
	c.overrideParams[templateK8sRegistry] = util.GetRegistry(envVarK8sRegistry, defaultK8sRegistry)
	log.Infof("Using container registry: %s", c.overrideParams[templateK8sRegistry])
}

//ValidateParams validates params of an addon
func (c *AutoScalerAwsClient) ValidateParams() (bool, error) {

	params := []string{"clusterUUID", "clusterRegion", "cpuLimit", "memLimit", "cpuRequest", "memRequest"}

	for _, p := range params {
		if _, ok := c.overrideParams[p]; !ok {
			return false, addonerr.InvalidParams(p)
		}
	}

	return true, nil
}

//Health checks health of the instance
func (c *AutoScalerAwsClient) Health() (bool, error) {
	deploy, err := util.GetDeployment(casAWSNS, casAWSDeploy, c.client)
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

//Upgrade upgrades an CAutoScalerAws instance
func (c *AutoScalerAwsClient) Upgrade() error {
	return c.Install()
}

//Install installs an CAutoScalerAws instance
func (c *AutoScalerAwsClient) Install() error {

	b, err := util.CheckClusterUpgrading(c.client)
	if err != nil {
		return err
	}

	if b {
		return fmt.Errorf("Cluster is upgrading ignoring request")
	}

	inputPath, outputPath, err := util.EnsureDirStructure(casAWSDir, c.version)
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

//Uninstall removes an CAutoScalerAws instance
func (c *AutoScalerAwsClient) Uninstall() error {

	b, err := util.CheckClusterUpgrading(c.client)
	if err != nil {
		return err
	}

	if b {
		return fmt.Errorf("Cluster is upgrading ignoring request")
	}

	inputPath, outputPath, err := util.EnsureDirStructure(casAWSDir, c.version)
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
