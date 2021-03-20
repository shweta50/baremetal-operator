package addons

import (
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeVirtNS  = "kube-system"
	kubeVirtDir = "kubevirt"
)

// KubeVirtClient represents implementation for interacting with plain K8s cluster
type KubeVirtClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newKubeVirt(c client.Client, version string, params map[string]interface{}) *KubeVirtClient {

	cl := &KubeVirtClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//ValidateParams validates params of an addon
func (c *KubeVirtClient) ValidateParams() (bool, error) {

	return true, nil
}

//Health checks health of the instance
func (c *KubeVirtClient) Health() (bool, error) {
	return true, nil
}

//Upgrade upgrades an KubeVirt instance
func (c *KubeVirtClient) Upgrade() error {
	return c.Install()
}

//Install installs an KubeVirt instance
func (c *KubeVirtClient) Install() error {

	inputPath, outputPath, err := util.EnsureDirStructure(kubeVirtDir, c.version)
	if err != nil {
		return err
	}

	yamlList := []string{"kubevirt-operator.yaml", "kubevirt-cr.yaml", "cdi-operator.yaml", "cdi-cr.yaml"}

	for _, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.install(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
	}

	return nil
}

//Uninstall removes an KubeVirt instance
func (c *KubeVirtClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(kubeVirtDir, c.version)
	if err != nil {
		return err
	}

	yamlList := []string{"kubevirt-cr.yaml", "cdi-operator.yaml", "kubevirt-operator.yaml", "cdi-cr.yaml"}

	for _, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.uninstall(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
	}
	return nil
}

func (c *KubeVirtClient) install(inputFilePath, outputFilePath, fileName string) error {

	err := util.WriteConfigToTemplate(inputFilePath, outputFilePath, fileName, c.overrideParams)
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

func (c *KubeVirtClient) uninstall(inputFilePath, outputFilePath, fileName string) error {

	err := util.WriteConfigToTemplate(inputFilePath, outputFilePath, fileName, c.overrideParams)
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
