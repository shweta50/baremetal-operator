package addons

import (
	"os"
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	profileAgentNs         = "platform9-system"
	profileAgentDeployment = "pf9-profile-agent"
	profileAgentDir        = "pf9-profile-agent"
)

// ProfileAgentClient represents the pf9-profile-agent addon
type ProfileAgentClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newProfileAgent(c client.Client, version string, params map[string]interface{}) *ProfileAgentClient {

	cl := &ProfileAgentClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//overrideRegistry checks if we need to override container registry values
func (c *ProfileAgentClient) overrideRegistry() {
	overrideRegistry := util.GetRegistry(envVarDockerRegistry, "docker.io")
	c.overrideParams[templateDockerRegistry] = overrideRegistry

	log.Infof("Using container registry: %s", c.overrideParams[templateDockerRegistry])
}

//ValidateParams validates params of an addon
func (c *ProfileAgentClient) ValidateParams() (bool, error) {

	return true, nil
}

//Health checks health of the pf9-profile-agent instance
func (c *ProfileAgentClient) Health() (bool, error) {
	deploy, err := util.GetDeployment(profileAgentNs, profileAgentDeployment, c.client)
	if err != nil {
		log.Errorf("Failed to get deployment: %s", err)
		return false, err
	}

	if deploy == nil {
		return false, nil
	}

	if deploy.Status.ReadyReplicas > 0 {
		return true, nil
	}

	return false, nil
}

//Upgrade upgrades an pf9-profile-agent instance
func (c *ProfileAgentClient) Upgrade() error {
	return c.Install()
}

//Install installs an pf9-profile-agent instance
func (c *ProfileAgentClient) Install() error {

	inputPath, outputPath, err := util.EnsureDirStructure(profileAgentDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "pf9-profile-agent.yaml")
	outputFilePath := filepath.Join(outputPath, "pf9-profile-agent.yaml")

	c.overrideRegistry()

	// Fetch and add clusterID/projectID to the overrideParams
	clusterID := os.Getenv(util.ClusterIDEnvVar)
	projectID := os.Getenv(util.ProjectIDEnvVar)
	c.overrideParams["ClusterId"] = clusterID
	c.overrideParams["ProjectId"] = projectID

	labels := []util.Labels{}

	err = util.CreateNsIfNeeded(profileAgentNs, labels, c.client)
	if err != nil {
		log.Errorf("Failed to create ns: %s %s", profileAgentNs, err)
		return err
	}

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, "pf9-profile-agent.yaml", c.overrideParams)
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

//Uninstall removes an dashboard instance
func (c *ProfileAgentClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(profileAgentDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "pf9-profile-agent.yaml")
	outputFilePath := filepath.Join(outputPath, "pf9-profile-agent.yaml")

	c.overrideRegistry()

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, "pf9-profile-agent.yaml", c.overrideParams)
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
