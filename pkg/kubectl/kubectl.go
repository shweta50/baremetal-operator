package kubectl

/*
 *      Copyright 2020 Platform9, Inc.
 *      All rights reserved
 */

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	log "github.com/sirupsen/logrus"
)

const (
	kubectlPath = "/usr/local/bin/kubectl"
	kubeConfig  = "/root/.kube/config"
)

//DeployStatus stores the output of kubectl get deploy cmd
type DeployStatus struct {
	Status struct {
		AvailableReplicas int `yaml:"availableReplicas"`
	}
}

//InstallPkg installs kudo pkg
func InstallPkg(pkgname, version, ns string) error {
	log.Infof("Installing kudo pkg: %s", pkgname)

	cmd := exec.Command(getKubeCmd(), "kudo", "install", pkgname, "--operator-version", version, "--namespace", ns)

	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeConfig))

	var errb, outb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdout = &outb

	cmd.Start()

	if err := cmd.Wait(); err != nil {
		output := errb.String() + outb.String()
		log.Errorf("Error installing kudo pkg %s , output: %s", pkgname, output)
		return fmt.Errorf("%s", output)
	}

	log.Infof("Installed kudo pkg: %s", pkgname)
	return nil
}

//UninstallPkg uninstalls kudo pkg
func UninstallPkg(pkgname, ns string) error {
	log.Infof("Uninstalling kudo pkg: %s", pkgname)

	name := fmt.Sprintf("--instance=%s-instance", pkgname)

	cmd := exec.Command(getKubeCmd(), "kudo", "uninstall", name, "--namespace", ns)

	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeConfig))

	var errb, outb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdout = &outb

	cmd.Start()

	if err := cmd.Wait(); err != nil {
		output := errb.String() + outb.String()
		log.Errorf("Error uninstalling kudo pkg: %s, output: %s", pkgname, output)
		return err
	}

	log.Infof("Uninstalled kudo pkg: %s", pkgname)
	return nil
}

//UpgradePkg upgrades kudo pkg
func UpgradePkg(pkgname, version, ns string) error {
	log.Infof("Upgrading kudo pkg: %s", pkgname)

	name := fmt.Sprintf("--instance=%s-instance", pkgname)

	cmd := exec.Command(getKubeCmd(), "kudo", "upgrade", pkgname, name, "--operator-version", version, "--namespace", ns)

	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeConfig))

	var errb, outb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdout = &outb

	cmd.Start()

	if err := cmd.Wait(); err != nil {
		output := errb.String() + outb.String()
		log.Errorf("Error upgrading kudo pkg: %s, output: %s", pkgname, output)
		return err
	}

	log.Infof("Upgraded kudo pkg: %s", pkgname)
	return nil
}

func getKubeCmd() string {
	switch runtime.GOOS {
	case "linux":
		return kubectlPath
	case "darwin":
		return "kubectl"
	default:
		log.Panicf("Unsupported OS: %s", runtime.GOOS)
	}

	return "kubectl"
}
