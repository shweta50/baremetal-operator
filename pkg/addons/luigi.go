package addons

import (
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	luigiNS     = "luigi-system"
	luigiDir    = "luigi"
	luigiDeploy = "luigi-controller-manager"
)

// LuigiClient represents implementation for interacting with plain K8s cluster
type LuigiClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newLuigi(c client.Client, version string, params map[string]interface{}) *LuigiClient {

	cl := &LuigiClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//overrideRegistry checks if we need to override container registry values
func (c *LuigiClient) overrideRegistry() {
	dockerRegistry := util.GetRegistry(envVarDockerRegistry, defaultDockerRegistry)
	gcrRegistry := util.GetRegistry(envVarGcrRegistry, defaultGcrRegistry)

	if dockerRegistry != "" {
		c.overrideParams[templateDockerRegistry] = dockerRegistry
	}

	if gcrRegistry != "" {
		c.overrideParams[templateGcrRegistry] = gcrRegistry
	}

	log.Infof("Using container registry: %s, %s", c.overrideParams[templateDockerRegistry], c.overrideParams[templateGcrRegistry])
}

//ValidateParams validates params of an addon
func (c *LuigiClient) ValidateParams() (bool, error) {

	return true, nil
}

//Health checks health of the instance
func (c *LuigiClient) Health() (bool, error) {

	deploy, err := util.GetDeployment(luigiNS, luigiDeploy, c.client)
	if err != nil {
		log.Errorf("Failed to get deployment: %s", err)
		return false, err
	}

	if deploy == nil {
		return false, nil
	}

	return true, nil
}

//Upgrade upgrades an luigi instance
func (c *LuigiClient) Upgrade() error {
	return c.Install()
}

//Install installs an luigi instance
func (c *LuigiClient) Install() error {

	labels := []util.Labels{
		util.Labels{
			Key:   "control-plane",
			Value: "controller-manager",
		},
	}

	err := util.CreateNsIfNeeded(luigiNS, labels, c.client)
	if err != nil {
		log.Errorf("Failed to create ns: %s %s", luigiNS, err)
		return err
	}

	inputPath, outputPath, err := util.EnsureDirStructure(luigiDir, c.version)
	if err != nil {
		return err
	}

	c.overrideRegistry()

	fileName := "luigi.yaml"

	inputFilePath := filepath.Join(inputPath, fileName)
	outputFilePath := filepath.Join(outputPath, fileName)

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, fileName, c.overrideParams)
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

//Uninstall removes an luigi instance
func (c *LuigiClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(luigiDir, c.version)
	if err != nil {
		return err
	}

	c.overrideRegistry()

	fileName := "luigi.yaml"

	inputFilePath := filepath.Join(inputPath, fileName)
	outputFilePath := filepath.Join(outputPath, fileName)

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, fileName, c.overrideParams)
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
