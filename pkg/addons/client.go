package addons

import (
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client handles installation of specific Addons
type Client interface {
	Health() (bool, error)
	ValidateParams() (bool, error)
	Upgrade() error
	Install() error
	Uninstall() error
}

// Get returns an Addon instance
func getAddonClient(mode, version string, params map[string]interface{}, c client.Client) Client {
	var instance Client

	switch mode {
	case "coredns":
		instance = newCoreDNS(c, version, params)
	case "metallb":
		instance = newMetalLB(c, version, params)
	case "kubernetes-dashboard":
		instance = newKubeDashboard(c, version, params)
	case "metrics-server":
		instance = newMetricsServer(c, version, params)
	case "cluster-auto-scaler-aws":
		instance = newAutoScalerAws(c, version, params)
	case "cluster-auto-scaler-azure":
		instance = newAutoScalerAzure(c, version, params)
	case "kubevirt":
		instance = newKubeVirt(c, version, params)
	case "monitoring":
		instance = newMonitoring(c, version, params)
	case "luigi":
		instance = newLuigi(c, version, params)
	case "pf9-profile-agent":
		instance = newProfileAgent(c, version, params)

	default:
		log.Errorf("Mode %s is not supported", mode)
	}

	return instance
}
