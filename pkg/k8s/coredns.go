package k8s

import (
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	corednsNS     = "kube-system"
	corednsSecret = "memberlist"
	dnsDir        = "coredns"
	corednsDeploy = "coredns"
)

// CoreDNSClient represents implementation for interacting with plain K8s cluster
type CoreDNSClient struct {
	c              client.Client
	overrideParams map[string]interface{}
	version        string
}

func getCoreDNS(c client.Client, version string, params map[string]interface{}) *CoreDNSClient {

	cl := &CoreDNSClient{
		c:              c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//Health checks health of the instance
func (c *CoreDNSClient) Health() (bool, error) {

	d, err := util.GetDeployment(corednsNS, corednsDeploy, c.c)
	if err != nil {
		log.Errorf("Failed to get rc: %s", err)
		return false, err
	}

	if d.Status.ReadyReplicas > 0 {
		return true, nil
	}

	return false, nil
}

//Upgrade upgrades an coredns instance
func (c *CoreDNSClient) Upgrade() error {
	return c.Install()
}

//Install installs an coredns instance
func (c *CoreDNSClient) Install() error {
	inputPath, outputPath, err := util.EnsureDirStructure(dnsDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "coredns.yaml")
	outputFilePath := filepath.Join(outputPath, "coredns.yaml")

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

//Uninstall removes an coredns instance
func (c *CoreDNSClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(dnsDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "coredns.yaml")
	outputFilePath := filepath.Join(outputPath, "coredns.yaml")

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
