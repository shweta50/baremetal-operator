package addons

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
	"github.com/platform9/pf9-addon-operator/pkg/util"
)

const (
	monitoringNS     = "pf9-monitoring"
	operatorsNS      = "pf9-operators"
	olmNS            = "pf9-olm"
	defaultDashboard = "grafana-dashboard-cluster-explorer"
	monitoringDir    = "monitoring"
	promSFS          = "prometheus-system"
	tokenFile        = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	grafanaURL       = "https://localhost:443/api/v1/namespaces/pf9-monitoring/services/http:grafana-ui:80/proxy/"
)

// MonitoringClient represents implementation for monitoring
type MonitoringClient struct {
	client         client.Client
	overrideParams map[string]interface{}
	version        string
}

func newMonitoring(c client.Client, version string, params map[string]interface{}) *MonitoringClient {

	cl := &MonitoringClient{
		client:         c,
		overrideParams: params,
		version:        version,
	}

	return cl
}

//overrideRegistry checks if we need to override container registry values
func (c *MonitoringClient) overrideRegistry() {

	dockerRegistry := util.GetRegistry(envVarDockerRegistry, defaultDockerRegistry)
	quayRegistry := util.GetRegistry(envVarQuayRegistry, defaultQuayRegistry)

	if dockerRegistry != "" {
		c.overrideParams[templateDockerRegistry] = dockerRegistry
	}

	if quayRegistry != "" {
		c.overrideParams[templateQuayRegistry] = quayRegistry
	}

	log.Infof("Using container registry: %s %s", c.overrideParams[templateDockerRegistry], c.overrideParams[templateQuayRegistry])
}

func (c *MonitoringClient) install(inputFilePath, outputFilePath, fileName string) error {

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

func (c *MonitoringClient) uninstall(inputFilePath, outputFilePath, fileName string) error {

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

func (c *MonitoringClient) createSecret(inputPath, filename, ns, name, key string) error {

	file, err := os.Open(inputPath + "/promplus/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	val, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	s, err := util.GetSecret(ns, name, c.client)
	if err != nil {
		log.Errorf("Failed to get secret: %s %s", name, err)
		return err
	}

	if s != nil {
		log.Infof("Secret %s/%s already exists", ns, name)
		return nil
	}

	log.Debugf("Creating secret %s/%s", ns, name)
	err = util.CreateSecret(ns, name, key, val, c.client)
	if err != nil {
		log.Errorf("Failed to get secret: %s %s", name, err)
		return err
	}

	return nil
}

func (c *MonitoringClient) createConfigMap(inputPath, filename, ns, name, key string) error {
	file, err := os.Open(inputPath + "/promplus/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	val, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	cm, err := util.GetConfigMap(ns, name, c.client)
	if err != nil {
		log.Errorf("Failed to get configmap: %s %s", name, err)
		return err
	}

	if cm != nil {
		log.Infof("Configmap %s/%s already exists", ns, name)
		return nil
	}

	log.Debugf("Creating configmap %s/%s", ns, name)
	err = util.CreateConfigMap(ns, name, key, val, c.client)
	if err != nil {
		log.Errorf("Failed to get configmap: %s %s", name, err)
		return err
	}

	return nil
}

func (c *MonitoringClient) postUnInstall(inputPath string) error {
	var err error
	for _, cfgName := range []string{"grafana-dashboards", defaultDashboard, "grafana-conf", "nginx-conf", "grafana-dashboard-apiserver", "grafana-dashboard-events", "grafana-dashboard-fs", "grafana-dashboard-kubelet", "grafana-dashboard-kubernetes", "grafana-dashboard-memusage", "grafana-dashboard-network", "grafana-dashboard-node-exporter", "grafana-dashboard-pvc"} {
		if err = util.DeleteConfigMap(monitoringNS, cfgName, c.client); err != nil {
			log.Errorf("Failed to delete configmap: %s %s", cfgName, err)
			return err
		}
	}

	for _, secName := range []string{"scrapeconfig", "alertmanager-sysalert", "grafana-datasources"} {
		if err = util.DeleteSecret(monitoringNS, secName, c.client); err != nil {
			log.Errorf("Failed to delete secret: %s %s", secName, err)
			return err
		}
	}

	return nil
}

func (c *MonitoringClient) preInstall(inputPath string) error {
	var err error

	if err = util.CreateNsIfNeeded(monitoringNS, []util.Labels{}, c.client); err != nil {
		return err
	}

	if err = util.CreateNsIfNeeded(operatorsNS, []util.Labels{}, c.client); err != nil {
		return err
	}

	if err = c.createSecret(inputPath, "additional-scrape-config.yaml", monitoringNS, "scrapeconfig", "additional-scrape-config.yaml"); err != nil {
		return err
	}

	if err = c.createSecret(inputPath, "alertmanager.yaml", monitoringNS, "alertmanager-sysalert", "alertmanager.yaml"); err != nil {
		return err
	}

	if err = c.createSecret(inputPath, "grafana-datasources", monitoringNS, "grafana-datasources", "datasources.yaml"); err != nil {
		return err
	}

	if err = c.createConfigMap(inputPath, "grafana-dashboards", monitoringNS, "grafana-dashboards", "dashboards.yaml"); err != nil {
		return err
	}

	if err = c.createConfigMap(inputPath, defaultDashboard, monitoringNS, defaultDashboard, "home.json"); err != nil {
		return err
	}

	for _, cfgFile := range []string{"grafana-dashboard-apiserver", "grafana-dashboard-events", "grafana-dashboard-fs", "grafana-dashboard-kubelet", "grafana-dashboard-kubernetes", "grafana-dashboard-memusage", "grafana-dashboard-network", "grafana-dashboard-node-exporter", "grafana-dashboard-pvc"} {
		if err = c.createConfigMap(inputPath, cfgFile, monitoringNS, cfgFile, cfgFile+".json"); err != nil {
			return err
		}
	}

	if err = c.createConfigMap(inputPath, "nginx-config", monitoringNS, "nginx-conf", "nginx.conf"); err != nil {
		return err
	}

	if err = c.createConfigMap(inputPath, "grafana-config", monitoringNS, "grafana-conf", "grafana.ini"); err != nil {
		return err
	}

	return nil
}

//ValidateParams validates params of an addon
func (c *MonitoringClient) ValidateParams() (bool, error) {

	if _, ok := c.overrideParams["retentionTime"]; !ok {
		return false, addonerr.InvalidParams("retentionTime")
	}

	if _, ok := c.overrideParams["storageClassName"]; !ok {
		//storageClassName is optional, if not specified we don't need pvcSize either
		return true, nil
	}
	pvcSize, ok := c.overrideParams["pvcSize"]
	if !ok {
		return false, addonerr.InvalidParams("pvcSize")
	}

	pvcSizeStr := fmt.Sprintf("%v", pvcSize)

	if !strings.HasSuffix(pvcSizeStr, "Gi") {
		return false, addonerr.InvalidParams("pvcSize invalid / ")
	}

	return true, nil
}

//Health checks health of the instance
func (c *MonitoringClient) Health() (bool, error) {

	sfs, err := util.GetStatefulSet(monitoringNS, promSFS, c.client)
	if err != nil {
		log.Errorf("Failed to get statefulset: %s", err)
		return false, err
	}

	if sfs == nil {
		return false, nil
	}

	if sfs.Status.ReadyReplicas == 0 {
		return false, nil
	}

	text, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return false, err
	}

	var bearer = "Bearer " + strings.TrimSuffix(string(text), "\n")

	req, err := http.NewRequest("GET", grafanaURL, nil)
	if err != nil {
		return false, err
	}

	req.Header.Add("Authorization", bearer)

	cl := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := cl.Do(req)
	if err != nil {
		log.Errorf("Failed in invoke URL: %s, error: %s", grafanaURL, err)
		return false, err
	}

	defer resp.Body.Close()

	log.Debugf("Successfully invoked URL: %s, ret code: %d",
		grafanaURL, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Errorf("Monitoring health check failed, invoked URL: %s, ret code: %d, expected: %d",
			grafanaURL, resp.StatusCode, http.StatusOK)
		return false, nil
	}

	return true, nil
}

//Upgrade upgrades an metallb instance
func (c *MonitoringClient) Upgrade() error {
	return c.Install()
}

//Install installs an metallb instance
func (c *MonitoringClient) Install() error {
	c.overrideRegistry()

	inputPath, outputPath, err := util.EnsureDirStructure(monitoringDir, c.version)
	if err != nil {
		return err
	}

	if err = c.preInstall(inputPath); err != nil {
		log.Errorf("Failed in monitoring preinstall: %s", err)
		return err
	}

	yamlList := []string{"prometheus-operator-0.46.0.yaml", "monhelper.yaml", "objects.yaml", "grafana.yaml", "kube-state-metrics.yaml", "node-exporter.yaml"}

	for _, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.install(inputFilePath, outputFilePath, y); err != nil {
			return err
		}
	}

	inputFilePath := filepath.Join(inputPath, "prometheus-rules.yaml")
	err = util.ApplyYaml(inputFilePath, c.client)
	if err != nil {
		log.Errorf("Failed to apply yaml file: %s", err)
		return err
	}

	return nil
}

//Uninstall removes an metallb instance
func (c *MonitoringClient) Uninstall() error {

	// In case the existing monitoring stack was deployed with OLM, cleanup those objects
	util.DeleteObject(operatorsNS, "prometheusoperator.0.37.0", "clusterserviceversion", "operators.coreos.com/v1alpha1", c.client)
	util.DeleteObject(olmNS, "olm-operator", "Deployment", "apps/v1", c.client)
	util.DeleteObject(olmNS, "catalog-operator", "Deployment", "apps/v1", c.client)
	util.DeleteObject(olmNS, "packageserver", "clusterserviceversion", "operators.coreos.com/v1alpha1", c.client)
	util.DeleteObject(olmNS, "platform9-operators", "catalogsource", "operators.coreos.com/v1alpha1", c.client)
	util.DeleteConfigMap(olmNS, "appbert", c.client)

	c.overrideRegistry()

	inputPath, outputPath, err := util.EnsureDirStructure(monitoringDir, c.version)
	if err != nil {
		return err
	}

	inputFilePath := filepath.Join(inputPath, "prometheus-rules.yaml")
	err = util.DeleteYaml(inputFilePath, c.client)
	if err != nil {
		log.Errorf("Failed to delete yaml file: %s", err)
	}

	yamlList := []string{"objects.yaml", "kube-state-metrics.yaml", "node-exporter.yaml", "monhelper.yaml", "grafana.yaml", "prometheus-operator-0.46.0.yaml"}

	for _, y := range yamlList {
		inputFilePath := filepath.Join(inputPath, y)
		outputFilePath := filepath.Join(outputPath, y)

		if err := c.uninstall(inputFilePath, outputFilePath, y); err != nil {
			log.Errorf("Error deleting yaml: %s", y)
		}
	}

	if err = c.postUnInstall(inputPath); err != nil {
		log.Errorf("Failed in post uninstall for monitoring: %s", err)
	}

	return nil
}
