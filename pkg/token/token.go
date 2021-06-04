package token

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultCfgFile  = "/etc/addon/config.yaml"
	kubeCfgTemplate = "/etc/addon/keystone.kubeconfig.template"
)

type JWTClaims struct {
	Foo string `json:"foo"`
	jwt.StandardClaims
}

//GetSunpikeKubeCfg gets sunpike kubecfg for a specific cluster
func GetSunpikeKubeCfg(token, clusterID, project string) (*rest.Config, error) {

	duFqdn := os.Getenv(util.DuFqdnEnvVar)

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

//GetSunpikeToken gets a token to reach sunpike through qbert v5
func GetSunpikeToken() (string, error) {
	key := []byte("901C6739B76A4320B7C06578AF469F5A")
	// Create the Claims
	claims := JWTClaims{
		"2415D3FD3AE2",
		jwt.StandardClaims{
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Issuer:    "addon",
		},
	}

	encToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return encToken.SignedString(key)
}
