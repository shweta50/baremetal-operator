package k8s

import (
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	metricsServerNS  = "kube-system"
	metricsServerDir = "metrics-server"
)

// MetricsServerClient represents implementation for interacting with plain K8s cluster
type MetricsServerClient struct {
	c              client.Client
	overrideParams map[string]interface{}
	version        string
}

func getMetricsServer(c client.Client, version string, params map[string]interface{}) *MetricsServerClient {

	cl := &MetricsServerClient{
		c:              c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//Get retrieves a MetricsServer instance
func (c *MetricsServerClient) Get() error {
	return nil
}

//Upgrade upgrades an MetricsServer instance
func (c *MetricsServerClient) Upgrade() error {
	return c.Install()
}

//Install installs an MetricsServer instance
func (c *MetricsServerClient) Install() error {

	inputPath, outputPath, err := util.EnsureDirStructure(metricsServerDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "metrics-server.yaml")
	outputFilePath := filepath.Join(outputPath, "metrics-server.yaml")

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

//Uninstall removes an MetricsServer instance
func (c *MetricsServerClient) Uninstall() error {
	inputPath, outputPath, err := util.EnsureDirStructure(metricsServerDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "metrics-server.yaml")
	outputFilePath := filepath.Join(outputPath, "metrics-server.yaml")

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
