package addons

import (
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/platform9/pf9-addon-operator/pkg/util"
)

const (
	dnsScalerNS     = "kube-system"
	dnsScalerDir    = "dns-autoscaler"
	dnsScalerDeploy = "kube-dns-autoscaler"
)

// DNSScalerClient represents implementation for csi cinder
type DNSScalerClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newDNSScaler(c client.Client, version string, params map[string]interface{}) *DNSScalerClient {

	cl := &DNSScalerClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//overrideRegistry checks if we need to override container registry values
func (c *DNSScalerClient) overrideRegistry() {
	c.overrideParams[templateK8sRegistry] = util.GetRegistry(envVarK8sRegistry, defaultK8sRegistry)

	log.Infof("Using k8s registry: %s", c.overrideParams[templateK8sRegistry])
}

//ValidateParams validates params of an addon
func (c *DNSScalerClient) ValidateParams() (bool, error) {

	return true, nil
}

//Health checks health of the instance
func (c *DNSScalerClient) Health() (bool, error) {
	d, err := util.GetDeployment(dnsScalerNS, dnsScalerDeploy, c.client)
	if err != nil {
		log.Errorf("Failed to get rc: %s", err)
		return false, err
	}

	if d == nil {
		return false, nil
	}

	if d.Status.ReadyReplicas > 0 {
		return true, nil
	}

	return false, nil
}

//Upgrade upgrades an logging instance
func (c *DNSScalerClient) Upgrade() error {
	return c.Install()
}

//Install installs an logging instance
func (c *DNSScalerClient) Install() error {

	inputPath, outputPath, err := util.EnsureDirStructure(dnsScalerDir, c.version)
	if err != nil {
		return err
	}

	c.overrideRegistry()

	yamlList := []string{"cfgmap.yaml", "deploy.yaml"}

	for _, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.install(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
	}

	return nil
}

func (c *DNSScalerClient) install(inputFilePath, outputFilePath, fileName string) error {

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

//Uninstall removes an logging instance
func (c *DNSScalerClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(dnsScalerDir, c.version)
	if err != nil {
		return err
	}

	c.overrideRegistry()

	yamlList := []string{"cfgmap.yaml", "deploy.yaml"}

	for _, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.uninstall(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
	}

	return nil
}

func (c *DNSScalerClient) uninstall(inputFilePath, outputFilePath, fileName string) error {

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
