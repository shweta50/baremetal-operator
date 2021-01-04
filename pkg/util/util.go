package util

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/apply"
	"github.com/platform9/pf9-addon-operator/pkg/objects"
)

//EnsureDirStructure ensures expected dir structure
func EnsureDirStructure(name, version string) (string, string, error) {

	versionDir := name + "/" + version
	inputPath := filepath.Join(objects.TemplateDir, versionDir)
	outputPath := filepath.Join(objects.CreateDir, versionDir)

	if err := DirExists(inputPath); err != nil {
		return "", "", err
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return "", "", err
	}

	return inputPath, outputPath, nil
}

//DirExists checks if a dir exists
func DirExists(inputPath string) error {
	_, err := os.Stat(inputPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("Dir %s does not exist", inputPath)
	}
	return err
}

//GetOverrideParams puts all override params in a map
func GetOverrideParams(addon *agentv1.Addon) (map[string]interface{}, error) {

	manifestParams := map[string]interface{}{}

	for _, p := range addon.Spec.Override.Params {
		log.Debugf("Adding param %s:%s", p.Name, p.Value)
		if p.Name == "additionalDnsConfig" {
			b, _ := base64.StdEncoding.DecodeString(p.Value)
			p.Value = string(b)
			log.Debugf("Decoded param %s:%s", p.Name, p.Value)
		}
		manifestParams[p.Name] = p.Value
	}

	return manifestParams, nil
}

//WriteConfigToTemplate writes templatized yaml to output dir
func WriteConfigToTemplate(inputPath, outputPath string, params map[string]interface{}) error {
	t, err := template.ParseFiles(inputPath)
	if err != nil {
		return err
	}

	if err := renderTemplateToFile(params, t, outputPath); err != nil {
		return err
	}

	return nil
}

//ReadManifestFile reads the addon manifest file
func ReadManifestFile(path string) (map[string]objects.AddonState, error) {
	var addonList []objects.AddonState

	text, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorf("Failed to read file %s", path)
		return nil, err
	}
	if err := json.Unmarshal(text, &addonList); err != nil {
		return nil, err
	}

	addonState := map[string]objects.AddonState{}
	for _, a := range addonList {
		addonState[a.Type+"-"+a.Version] = a
	}

	return addonState, nil
}

//ApplyYaml on the specified path
func ApplyYaml(path string, c client.Client) error {
	text, err := ioutil.ReadFile(path)
	if err != nil {
		log.Infof("Failed to read file %s", path)
		return err
	}

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
		log.Infof("Applying %s name: %s", obj.GetKind(), obj.GetName())
		err := apply.ApplyObject(context.Background(), c, obj)
		if err != nil {
			log.Error(err, "Error applying unstructured object")
			return err
		}
	}
	return nil
}

//DeleteYaml on the specified path
func DeleteYaml(path string, c client.Client) error {
	text, err := ioutil.ReadFile(path)
	if err != nil {
		log.Infof("Failed to read file %s", path)
		return err
	}

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

	for i := len(resourceList) - 1; i >= 0; i-- {
		obj := resourceList[i]
		log.Infof("Deleting %s name: %s", obj.GetKind(), obj.GetName())
		err := apply.DeleteObject(context.Background(), c, obj)
		if err != nil {
			log.Error(err, "Error deleting unstructured object")
			return err
		}
	}
	return nil
}

//GenerateRandKey generate base64 encoded rand key
func GenerateRandKey(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

//GetSecret gets a secret
func GetSecret(ns, name string, c client.Client) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	err := c.Get(context.Background(), client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, secret)
	if err != nil {
		if k8serror.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return secret, nil
}

//CreateSecret creates a secret
func CreateSecret(ns, name, key, data string, c client.Client) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Data: map[string][]byte{
			key: []byte(data),
		},
	}

	err := c.Create(context.Background(), secret)
	if err != nil {
		return err
	}

	return nil
}

func renderTemplateToFile(config map[string]interface{}, t *template.Template, filename string) error {

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	err = t.Execute(f, config)
	if err != nil {
		fmt.Printf("template.Execute failed for file: %s err: %s\n", filename, err)
		f.Close()
		return err
	}
	f.Close()
	return nil
}
