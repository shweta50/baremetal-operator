package k8s

/*
 *      Copyright 2020 Platform9, Inc.
 *      All rights reserved
 */

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"

	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kudobuilder/kudo/pkg/engine/renderer"
	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/apply"
	"github.com/platform9/pf9-addon-operator/pkg/util"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	kubectlPath = "/usr/local/bin/kubectl"
	kubeConfig  = "/Users/mayureshk/.kube/config"
)

// Process processess an addon pkg
type Process struct {
	client.Client
}

//DeployStatus stores the output of kubectl get deploy cmd
type DeployStatus struct {
	Status struct {
		AvailableReplicas int `yaml:"availableReplicas"`
	}
}

//Parameter defines each parameters in manifest file
type Parameter struct {
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Default     string `json:"default"`
}

//Parameters is an array of Parameter
type Parameters []Parameter

// ManifestFile defines manifest file
type ManifestFile struct {
	Parameters Parameters `json:"parameters"`
}

func overrideParams(kdir string, addon *agentv1.Addon) (map[string]interface{}, error) {

	manifestParams, err := readManifestFile(kdir)
	if err != nil {
		return nil, err
	}

	for _, p := range addon.Spec.Override.Params {
		if v, ok := manifestParams[p.Name]; ok {
			log.Debugf("Overriding %s with %s", v, p.Value)
			manifestParams[p.Name] = p.Value
		}
	}

	return manifestParams, nil
}

func readManifestFile(kdir string) (map[string]interface{}, error) {
	content, err := ioutil.ReadFile(kdir + "/manifest.yaml")
	if err != nil {
		log.Errorf("Failed to read manifest file in %s %s", kdir, err)
		return nil, err
	}

	manifestFile := ManifestFile{}
	if err = yaml.Unmarshal(content, &manifestFile); err != nil {
		return nil, err
	}

	params := map[string]interface{}{}

	for _, p := range manifestFile.Parameters {
		params[p.Name] = p.Default
	}

	return params, err
}

func getYamlFile(kdir string) (string, error) {
	var matches []string
	pattern := "*.yaml"

	err := filepath.Walk(kdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("No yaml files found in %s", kdir)
	}

	return matches[0], nil
}

func renderYaml(kdir string, vals map[string]interface{}) (string, error) {
	fname, err := getYamlFile(kdir)
	if err != nil {
		log.Errorf("Failed to find yaml in dir: %s", kdir)
		return "", err
	}

	content, err := ioutil.ReadFile(fname)
	if err != nil {
		log.Errorf("Failed to read yaml %s", fname)
		return "", err
	}

	text := string(content)

	engine := renderer.New()

	rendered, err := engine.Render("test", text, vals)
	if err != nil {
		log.Errorf("error rendering template: %s", err)
		return "", err
	}

	return rendered, err
}

//InstallPkg installs addon pkg
func (w *Watcher) InstallPkg(addon *agentv1.Addon) error {
	pkgname := addon.Name + "-" + addon.Spec.Version
	log.Infof("Installing pkg: %s", pkgname)

	params, err := util.GetOverrideParams(addon)
	if err != nil {
		log.Errorf("Failed to override params: %s", err)
		return err
	}

	addonClient := getAddonClient(addon.Spec.Type, addon.Spec.Version, params, w.c)

	if err := addonClient.Install(); err != nil {
		log.Errorf("Error installing addon: %s", err)
		return err
	}

	log.Infof("Installed pkg: %s", pkgname)
	return nil
}

//UninstallPkg uninstalls kudo pkg
func (w *Watcher) UninstallPkg(addon *agentv1.Addon) error {
	pkgname := addon.Name + "-" + addon.Spec.Version
	log.Infof("UnInstalling pkg: %s", pkgname)

	params, err := util.GetOverrideParams(addon)
	if err != nil {
		log.Errorf("Failed to override params: %s", err)
		return err
	}

	addonClient := getAddonClient(addon.Spec.Type, addon.Spec.Version, params, w.c)

	if err := addonClient.Uninstall(); err != nil {
		log.Errorf("Error installing addon: %s", err)
		return err
	}

	log.Infof("UnInstalled pkg: %s", pkgname)
	return nil
}

//UpgradePkg upgrades kudo pkg
func (w *Watcher) UpgradePkg(addon *agentv1.Addon) error {
	pkgname := addon.Name + "-" + addon.Spec.Version
	log.Infof("Upgrading pkg: %s", pkgname)

	params, err := util.GetOverrideParams(addon)
	if err != nil {
		log.Errorf("Failed to override params: %s", err)
		return err
	}

	addonClient := getAddonClient(addon.Spec.Type, addon.Spec.Version, params, w.c)

	if err := addonClient.Upgrade(); err != nil {
		log.Errorf("Error upgrading addon: %s", err)
		return err
	}

	log.Infof("Upgraded pkg: %s", pkgname)
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

func (w *Watcher) installPkg(text string, addon *agentv1.Addon) error {

	resourceList := []*unstructured.Unstructured{}
	decoder := apiyaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(text)), 4096)
	for {
		resource := unstructured.Unstructured{}
		err := decoder.Decode(&resource)
		if err == nil {
			resourceList = append(resourceList, &resource)
		} else if err == io.EOF {
			break
		} else {
			log.Error("Error decoding to unstructured", err)
			return err
		}
	}

	for _, obj := range resourceList {
		log.Infof("Creating %s name: %s", obj.GetKind(), obj.GetName())
		err := apply.ApplyObject(context.Background(), w.c, obj)
		if err != nil {
			log.Error(err, "Error applying unstructured object")
			return err
		}
	}

	return nil
}

/*func (w *Watcher) uninstallPkg(kdir string, addon *agentv1.Addon) error {
	pkgname := addon.Name

	patchFile := kdir + "/patch.yaml"
	exist := doesFileExist(patchFile)
	if exist {
		b, err := ioutil.ReadFile(patchFile)
		if err != nil {
			log.Error("Failed to read patch file", err)
			return err
		}

		patchStr := string(b)
		for _, p := range addon.Spec.Override.Params {
			log.Debugf("Params: %s  %s", p.Name, p.Value)
			r, _ := regexp.Compile(p.Name)
			patchStr = r.ReplaceAllString(patchStr, p.Value)
		}

		if err := ioutil.WriteFile(patchFile, []byte(patchStr), 0644); err != nil {
			log.Errorf("Failed to read patch file: %s %s", patchFile, err)
			return err
		}
	}

	if err := buildPkg(kdir); err != nil {
		log.Error("Failed to build kustomize pkg", err)
		return err
	}

	cmd := exec.Command(getKubeCmd(), "delete", "-f", kdir+"/output.yaml")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeConfig))

	var errb, outb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdout = &outb

	cmd.Start()

	if err := cmd.Wait(); err != nil {
		output := errb.String() + outb.String()
		log.Errorf("Error installing kustomize pkg %s , output: %s", pkgname, output)
		return fmt.Errorf("%s", output)
	}

	return nil
}

func doesFileExist(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return true
}*/
