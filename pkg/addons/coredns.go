package addons

import (
	"fmt"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/util"
)

const (
	corednsNS         = "kube-system"
	corednsSecret     = "memberlist"
	dnsDir            = "coredns"
	corednsDeploy     = "coredns"
	corednsRetryCount = 3
	corednsRetrySleep = 30
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

//overrideRegistry checks if we need to override container registry values
func (c *CoreDNSClient) overrideRegistry() {
	c.overrideParams[templateK8sRegistry] = util.GetRegistry(envVarK8sRegistry, defaultK8sRegistry)
	log.Infof("Using container registry: %s", c.overrideParams[templateK8sRegistry])
}

//ValidateParams validates params of an addon
func (c *CoreDNSClient) ValidateParams() (bool, error) {

	params := []string{"dnsDomain", "dnsMemoryLimit"}

	for _, p := range params {
		if _, ok := c.overrideParams[p]; !ok {
			return false, addonerr.InvalidParams(p)
		}
	}

	//If dnsServer is already specified no need to get it from addon-config
	if _, ok := c.overrideParams["dnsServer"]; ok {
		return true, nil
	}

	sec, err := util.GetSecret(addonsNS, addonsConfigSecret, c.client)
	if err != nil {
		log.Errorf("Failed to get addon-config secret: %s", err)
		return false, err
	}

	if sec == nil {
		log.Error("addon-config secret not found")
		return false, fmt.Errorf("addon-config secret not found")
	}

	dnsIP, ok := sec.Data["dnsIP"]
	if !ok {
		log.Error("dnsIP not found in addon-config")
		return false, fmt.Errorf("dnsIP not found in addon-config")
	}

	c.overrideParams["dnsServer"] = string(dnsIP)

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

	if d == nil {
		return false, nil
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
	var err error
	// CoreDNS is a critical addon, it's installation should be retried a few times before giving up,
	// other addons can error out in the first attempt and can retried again from the UI
	// Retries for coredns might be most relevant when there is a transient issue during bootstrap, (etcd not available)
	// where a retry will be useful, otherwise if coredns does not come up, the cluster will not be useful
	for i := 0; i < corednsRetryCount; i++ {
		if err = c.install(); err == nil {
			return nil
		}

		log.Errorf("Error installing coredns: %s, count: %d or %d", err, i+1, corednsRetryCount)
		time.Sleep(time.Duration(corednsRetrySleep) * time.Second)
	}
	return err
}

func (c *CoreDNSClient) install() error {
	inputPath, outputPath, err := util.EnsureDirStructure(dnsDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "coredns.yaml")
	outputFilePath := filepath.Join(outputPath, "coredns.yaml")

	c.overrideRegistry()

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, "coredns.yaml", c.overrideParams)
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

	c.overrideRegistry()

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, "coredns.yaml", c.overrideParams)
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
