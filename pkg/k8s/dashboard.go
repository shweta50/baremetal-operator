package k8s

import (
	"fmt"
	"path/filepath"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dashboardNS        = "kubernetes-dashboard"
	dashboardSecret    = "kubernetes-dashboard-certs"
	dashboardSecretKey = "dashboard.key"
	dashboardSecretCrt = "dashboard.crt"
	dashboardDir       = "dashboard"
)

// DashboardClient represents the kube dashboard addon
type DashboardClient struct {
	c              client.Client
	overrideParams map[string]interface{}
	version        string
}

func getKubeDashboard(c client.Client, version string, params map[string]interface{}) *DashboardClient {

	cl := &DashboardClient{
		c:              c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//Health checks health of the instance
func (c *DashboardClient) Health() (bool, error) {
	return true, nil
}

//Upgrade upgrades an metallb instance
func (c *DashboardClient) Upgrade() error {
	return c.Install()
}

//Install installs an dashboard instance
func (c *DashboardClient) Install() error {
	inputPath, outputPath, err := util.EnsureDirStructure(dashboardDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "dashboard.yaml")
	outputFilePath := filepath.Join(outputPath, "dashboard.yaml")

	err = util.WriteConfigToTemplate(inputFilePath, outputFilePath, c.overrideParams)
	if err != nil {
		log.Errorf("Failed to write output file: %s", err)
		return err
	}

	err = c.preInstall()
	if err != nil {
		log.Errorf("Failed to process pre install for dashboard: %s", err)
		return err
	}

	err = util.ApplyYaml(outputFilePath, c.c)
	if err != nil {
		log.Errorf("Failed to apply yaml file: %s", err)
		return err
	}

	return nil
}

//Uninstall removes an dashboard instance
func (c *DashboardClient) Uninstall() error {

	inputPath, outputPath, err := util.EnsureDirStructure(dashboardDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "dashboard.yaml")
	outputFilePath := filepath.Join(outputPath, "dashboard.yaml")

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

func (c *DashboardClient) preInstall() error {
	sec, err := util.GetSecret(dashboardNS, dashboardSecret, c.c)
	if err != nil {
		log.Errorf("Failed to get secret: %s", err)
		return err
	}

	if sec == nil {
		return fmt.Errorf("Secret %s/%s not found", dashboardNS, dashboardSecret)
	} else {

		if _, ok := sec.Data[dashboardSecretKey]; !ok {
			return fmt.Errorf("Key: %s not found in secret %s/%s",
				dashboardSecretKey, dashboardNS, dashboardSecret)
		}

		if _, ok := sec.Data[dashboardSecretCrt]; !ok {
			return fmt.Errorf("Key: %s not found in secret %s/%s",
				dashboardSecretCrt, dashboardNS, dashboardSecret)
		}

		log.Infof("Secret %s/%s exists", dashboardNS, dashboardSecret)
	}

	return nil
}
