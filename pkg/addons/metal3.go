package addons

import (
	"path/filepath"
	"time"

	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	metal3Dir    = "metal3"
	metal3NS     = "baremetal-operator-system"
	metal3Deploy = "baremetal-operator-controller-manager"
)

// Metal3Client represents implementation for interacting with plain K8s cluster
type Metal3Client struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newMetal3(c client.Client, version string, params map[string]interface{}) *Metal3Client {

	cl := &Metal3Client{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//overrideRegistry checks if we need to override container registry values
func (c *Metal3Client) overrideRegistry() {
	quayRegistry := util.GetRegistry(envVarQuayRegistry, defaultQuayRegistry)
	gcrRegistry := util.GetRegistry(envVarGcrRegistry, defaultGcrRegistry)

	if quayRegistry != "" {
		c.overrideParams[templateQuayRegistry] = quayRegistry
	}

	if gcrRegistry != "" {
		c.overrideParams[templateGcrRegistry] = gcrRegistry
	}

	log.Infof("Using container registry: %s, %s", c.overrideParams[templateQuayRegistry], c.overrideParams[templateGcrRegistry])
}

//ValidateParams validates params of an addon
func (c *Metal3Client) ValidateParams() (bool, error) {

	if _, ok := c.overrideParams["Metal3DhcpInterface"]; !ok {
		return false, addonerr.InvalidParams("Metal3DhcpInterface")
	}
	if _, ok := c.overrideParams["Metal3DhcpRange"]; !ok {
		return false, addonerr.InvalidParams("Metal3DhcpRange")
	}
	if _, ok := c.overrideParams["Metal3IronicHostIP"]; !ok {
		return false, addonerr.InvalidParams("Metal3IronicHostIP")
	}

	//TODO:
	if _, ok := c.overrideParams["deployDatabase"]; !ok {
		//storageClassName is optional, if not specified we deploy mysql db
		return true, nil
	}
	return true, nil
}

//Health checks health of the instance
func (c *Metal3Client) Health() (bool, error) {

	deploy, err := util.GetDeployment(metal3NS, metal3Deploy, c.client)
	if err != nil {
		log.Errorf("Failed to get deployment: %s", err)
		return false, err
	}

	if deploy == nil {
		return false, nil
	}

	return true, nil
}

//Upgrade upgrades an metal3 instance
func (c *Metal3Client) Upgrade() error {
	return c.Install()
}

//Install installs an metal3 instance
func (c *Metal3Client) Install() error {

	labels := []util.Labels{
		util.Labels{
			Key:   "control-plane",
			Value: "controller-manager",
		},
	}

	err := util.CreateNsIfNeeded(metal3NS, labels, c.client)
	if err != nil {
		log.Errorf("Failed to create ns: %s %s", metal3NS, err)
		return err
	}

	inputPath, outputPath, err := util.EnsureDirStructure(metal3Dir, c.version)
	if err != nil {
		return err
	}

	c.overrideRegistry()

	yamlList := []string{"cert-manager.yaml", "configmap.yaml", "ironic.yaml", "bmo-cert.yaml", "bmo.yaml"}

	for _, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.install(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
		// Wait for cert-manager or other services to come up
		time.Sleep(time.Duration(15) * time.Second)
	}

	return nil
}

//Uninstall removes an metal3 instance
func (c *Metal3Client) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(metal3Dir, c.version)
	if err != nil {
		return err
	}

	c.overrideRegistry()

	yamlList := []string{"bmo.yaml", "bmo-cert.yaml", "ironic.yaml", "configmap.yaml", "cert-manager.yaml"}

	for index, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.uninstall(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
		// Wait after deleting CRs, to allow the operator to cleanup resources
		if index == 1 {
			time.Sleep(time.Duration(30) * time.Second)
		}
	}

	// Cleanup resources deployed by metal3 that the operator may not have
	return nil
}

func (c *Metal3Client) install(inputFilePath, outputFilePath, fileName string) error {

	err := util.WriteConfigToTemplate(inputFilePath, outputFilePath, fileName, c.overrideParams)
	if err != nil {
		log.Errorf("Failed to write output file: %s", err)
		return err
	}

	log.Info("Metal3 addon installing: ", outputFilePath)
	err = util.ApplyYaml(outputFilePath, c.client)
	if err != nil {
		log.Errorf("Failed to apply yaml file: %s", err)
		return err
	}

	return nil
}

func (c *Metal3Client) uninstall(inputFilePath, outputFilePath, fileName string) error {

	err := util.WriteConfigToTemplate(inputFilePath, outputFilePath, fileName, c.overrideParams)
	if err != nil {
		log.Errorf("Failed to write output file: %s", err)
		return err
	}

	log.Info("Metal3 addon deleting: ", outputFilePath)
	err = util.DeleteYaml(outputFilePath, c.client)
	if err != nil {
		log.Errorf("Failed to delete yaml file: %s", err)
		return err
	}

	return nil
}
