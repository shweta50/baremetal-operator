package k8s

import (
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	casAWSNS  = "kube-system"
	casAWSDir = "cluster-autoscaler/aws"
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

//Get retrieves a CAutoScalerAws instance
func (c *CAutoScalerAwsClient) Get() error {
	return nil
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
