package k8s

import (
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	metallbNS        = "metallb-system"
	metallbSecret    = "memberlist"
	metallbSecretKey = "secretkey"
	metallbDir       = "metallb"
)

// MetallbClient represents implementation for interacting with plain K8s cluster
type MetallbClient struct {
	c              client.Client
	overrideParams map[string]interface{}
	version        string
}

func getMetalLB(c client.Client, version string, params map[string]interface{}) *MetallbClient {

	cl := &MetallbClient{
		c:              c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//Health checks health of the instance
func (c *MetallbClient) Health() (bool, error) {
	return true, nil
}

//Upgrade upgrades an metallb instance
func (c *MetallbClient) Upgrade() error {
	return c.Install()
}

//Install installs an metallb instance
func (c *MetallbClient) Install() error {

	inputPath, outputPath, err := util.EnsureDirStructure(metallbDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "metallb.yaml")
	outputFilePath := filepath.Join(outputPath, "metallb.yaml")

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

	err = c.postInstall()
	if err != nil {
		log.Errorf("Failed to process post install for metallb: %s", err)
		return err
	}

	return nil
}

//Uninstall removes an metallb instance
func (c *MetallbClient) Uninstall() error {
	inputPath, outputPath, err := util.EnsureDirStructure(metallbDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "metallb.yaml")
	outputFilePath := filepath.Join(outputPath, "metallb.yaml")

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

func (c *MetallbClient) postInstall() error {
	sec, err := util.GetSecret(metallbNS, metallbSecret, c.c)
	if err != nil {
		log.Errorf("Failed to get secret: %s", err)
		return err
	}

	if sec == nil {
		log.Info("Secret member list not found, creating it")

		val, err := util.GenerateRandKey(10)
		if err != nil {
			log.Errorf("Failed to generate rand key %s", err)
			return err
		}

		err = util.CreateSecret(metallbNS, metallbSecret, metallbSecretKey, val, c.c)
		if err != nil {
			log.Errorf("Failed to get secret: %s", err)
			return err
		}
	} else {
		log.Info("Secret member list already exists")
	}

	return nil
}
