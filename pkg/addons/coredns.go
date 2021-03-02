package addons

import (
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/util"
)

const (
	corednsNS     = "kube-system"
	corednsSecret = "memberlist"
	dnsDir        = "coredns"
	corednsDeploy = "coredns"
)

// CoreDNSClient represents implementation for interacting with plain K8s cluster
type CoreDNSClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newCoreDNS(c client.Client, version string, params map[string]interface{}) *CoreDNSClient {

	cl := &CoreDNSClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//ValidateParams validates params of an addon
func (c *CoreDNSClient) ValidateParams() (bool, error) {

	if _, ok := c.overrideParams["dnsDomain"]; !ok {
		return false, addonerr.InvalidParams("dnsDomain")
	}

	if _, ok := c.overrideParams["dnsMemoryLimit"]; !ok {
		return false, addonerr.InvalidParams("dnsMemoryLimit")
	}

	if _, ok := c.overrideParams["dnsServer"]; !ok {
		return false, addonerr.InvalidParams("dnsServer")
	}

	if b, ok := c.overrideParams["enableAdditionalDnsConfig"]; ok {
		enable, isStr := b.(string)
		if isStr && enable == "true" {
			if _, ok := c.overrideParams["base64EncAdditionalDnsConfig"]; !ok {
				return false, addonerr.InvalidParams("AdditionalDnsConfig")
			}
		}
	} else {
		return false, addonerr.InvalidParams("enableAdditionalDnsConfig")
	}

	return true, nil
}

//Health checks health of the instance
func (c *CoreDNSClient) Health() (bool, error) {
	//return true, nil
	d, err := util.GetDeployment(corednsNS, corednsDeploy, c.client)
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

	err = util.ApplyYaml(outputFilePath, c.client)
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

	err = util.DeleteYaml(outputFilePath, c.client)
	if err != nil {
		log.Errorf("Failed to delete yaml file: %s", err)
		return err
	}

	return nil
}
