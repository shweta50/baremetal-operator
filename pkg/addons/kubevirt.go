package addons

import (
	"path/filepath"
	"time"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeVirtCDINS          = "cdi"
	kubeVirtNS             = "kubevirt"
	kubeVirtDir            = "kubevirt"
	kubeVirtCDIDeploy      = "cdi-operator"
	kubeVirtOperatorDeploy = "virt-operator"
	// Resources to be cleaned up during uninstall
	virtAPIDeploy        = "virt-api"
	virtControllerDeploy = "virt-controller"
	virtHandlerDaemonset = "virt-handler"
	cdiAPIDeploy         = "cdi-apiserver"
	cdiDeploy            = "cdi-deployment"
	cdiProxyDeploy       = "cdi-uploadproxy"
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

//overrideRegistry checks if we need to override container registry values
func (c *KubeVirtClient) overrideRegistry() {
	overrideRegistry := util.GetRegistry(envVarQuayRegistry, defaultQuayRegistry)
	if overrideRegistry != "" {
		c.overrideParams[templateQuayRegistry] = overrideRegistry
	}

	log.Infof("Using container registry: %s", c.overrideParams[templateQuayRegistry])
}

//ValidateParams validates params of an addon
func (c *KubeVirtClient) ValidateParams() (bool, error) {

	return true, nil
}

//Health checks health of the instance
func (c *KubeVirtClient) Health() (bool, error) {

	deployCDI, err := util.GetDeployment(kubeVirtCDINS, kubeVirtCDIDeploy, c.client)
	if err != nil {
		log.Errorf("Failed to get deployment: %s", err)
		return false, err
	}

	deployOperator, err := util.GetDeployment(kubeVirtNS, kubeVirtOperatorDeploy, c.client)
	if err != nil {
		log.Errorf("Failed to get deployment: %s", err)
		return false, err
	}

	if deployCDI == nil || deployOperator == nil {
		return false, nil
	}

	return true, nil
}

//Upgrade upgrades an KubeVirt instance
func (c *KubeVirtClient) Upgrade() error {
	return c.Install()
}

//Install installs an KubeVirt instance
func (c *KubeVirtClient) Install() error {

	cdiLabels := []util.Labels{
		util.Labels{
			Key:   "cdi.kubevirt.io",
			Value: "",
		},
	}

	kvLabels := []util.Labels{
		util.Labels{
			Key:   "kubevirt.io",
			Value: "",
		},
	}

	err := util.CreateNsIfNeeded(kubeVirtNS, kvLabels, c.client)
	if err != nil {
		log.Errorf("Failed to create ns: %s %s", kubeVirtNS, err)
		return err
	}

	err = util.CreateNsIfNeeded(kubeVirtCDINS, cdiLabels, c.client)
	if err != nil {
		log.Errorf("Failed to create ns: %s %s", kubeVirtCDINS, err)
		return err
	}

	inputPath, outputPath, err := util.EnsureDirStructure(kubeVirtDir, c.version)
	if err != nil {
		return err
	}

	c.overrideRegistry()

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

	c.overrideRegistry()

	yamlList := []string{"kubevirt-cr.yaml", "cdi-cr.yaml", "cdi-operator.yaml", "kubevirt-operator.yaml"}

	for index, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.uninstall(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
		// Wait after deleting CRs, to allow the operator to cleanup resources
		if index == 1 {
			time.Sleep(30 * time.Second)
		}
	}

	// Cleanup resources deployed by kubevirt that the operator may not have
	util.DeleteDeployment(kubeVirtCDINS, cdiAPIDeploy, c.client)
	util.DeleteDeployment(kubeVirtCDINS, cdiDeploy, c.client)
	util.DeleteDeployment(kubeVirtCDINS, cdiProxyDeploy, c.client)

	util.DeleteDeployment(kubeVirtNS, virtAPIDeploy, c.client)
	util.DeleteDeployment(kubeVirtNS, virtControllerDeploy, c.client)
	util.DeleteDaemonset(kubeVirtNS, virtHandlerDaemonset, c.client)

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
