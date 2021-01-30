package k8s

import (
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client handles installation of specific Addons
type Client interface {
	Health() (bool, error)
	Upgrade() error
	Install() error
	Uninstall() error
}

// Get returns an Addon instance
func getAddonClient(mode, version string, params map[string]interface{}, c client.Client) Client {
	var instance Client

	switch mode {
	case "coredns":
		instance = getCoreDNS(c, version, params)
	case "metallb":
		instance = getMetalLB(c, version, params)
	case "kubernetes-dashboard":
		instance = getKubeDashboard(c, version, params)
	case "metrics-server":
		instance = getMetricsServer(c, version, params)
	case "cluster-auto-scaler-aws":
		instance = getCAutoScalerAws(c, version, params)
	case "cluster-auto-scaler-azure":
		instance = getCAutoScalerAzure(c, version, params)
	case "kubevirt":
		instance = getKubeVirt(c, version, params)

	default:
		log.Panicf("Mode %s is not supported", mode)
	}

	return instance
}
