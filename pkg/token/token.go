package token

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultCfgFile  = "/etc/addon/config.yaml"
	kubeCfgTemplate = "/etc/addon/keystone.kubeconfig.template"
)

func getSunpikeKubeCfg(ctx context.Context, clusterID, project string) (string, error) {
	kubeCfgPath := clusterID + ".cfg"

	// Check if the token has expired, if not use existing kubeconfig file
	if tokenCache.keystoneToken != "" && time.Now().Before(tokenCache.expires) {
		if _, err := os.Stat(kubeCfgPath); err == nil {
			return kubeCfgPath, nil
		}
	}

	keystoneAuthResult, err := getKeystoneToken(ctx, clusterID, project)
	if err != nil {
		return "", err
	}

	duFqdn := os.Getenv(util.DuFqdnEnvVar)

	data, err := ioutil.ReadFile(kubeCfgTemplate)
	if err != nil {
		log.Errorf("Failed to read kubecfg template: %s", err)
		return "", err
	}

	buf := strings.Replace(string(data), "__DU_QBERT_FQDN__", duFqdn, 1)
	buf = strings.Replace(buf, "__KEYSTONE_TOKEN__", keystoneAuthResult.Token, 1)
	buf = strings.Replace(buf, "__PROJECT_ID__", keystoneAuthResult.ProjectID, 1)

	err = ioutil.WriteFile(kubeCfgPath, []byte(buf), 0600)
	if err != nil {
		log.Errorf("Failed to get write kubecfg: %s", err)
		return "", err
	}

	return kubeCfgPath, nil
}

//GetSunpikeKubeCfg gets sunpike kubecfg for a specific cluster
func GetSunpikeKubeCfg(ctx context.Context, clusterID, project string) (*rest.Config, error) {

	kubeCfgPath, err := getSunpikeKubeCfg(ctx, clusterID, project)
	if err != nil {
		return nil, err
	}

	// If network is down the default config created here will block for a few minutes
	// while trying to list ClusterAddon objects from sunpike. We can explicitly specify
	// a timeout in case du_fqdn is not reachable using clientcmd.ConfigOverrides,
	// but that was not felt necessary, see PMK-3821 for details
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	if err != nil {
		return nil, err
	}

	return cfg, err
}
