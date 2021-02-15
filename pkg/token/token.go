package token

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultCfgFile  = "/etc/addon/config.yaml"
	kubeCfgTemplate = "/etc/addon/keystone.kubeconfig.template"
)

var duFqdn = getEnvDUFQDN()

//IsValidUUID check is the passed string is a valid UUID
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func getEnvDUFQDN() string {
	value, exists := os.LookupEnv("DU_FQDN")
	if !exists {
		panic(fmt.Sprintf("DU FQDN not defined as env variable"))
	}

	return value
}

//GetSunpikeKubeCfg gets sunpike kubecfg for a specific cluster
func GetSunpikeKubeCfg(token, clusterID, project string) (*rest.Config, error) {

	data, err := ioutil.ReadFile(kubeCfgTemplate)
	if err != nil {
		log.Errorf("Failed to read kubecfg template: %s", err)
		return nil, err
	}

	buf := strings.Replace(string(data), "__DU_QBERT_FQDN__", duFqdn, 1)
	buf = strings.Replace(buf, "__KEYSTONE_TOKEN__", token, 1)
	buf = strings.Replace(buf, "__PROJECT_ID__", project, 1)

	kubeCfgPath := clusterID + ".cfg"

	err = ioutil.WriteFile(kubeCfgPath, []byte(buf), 0600)
	if err != nil {
		log.Errorf("Failed to get write kubecfg: %s", err)
		return nil, err
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	if err != nil {
		return nil, err
	}

	return cfg, err
}
