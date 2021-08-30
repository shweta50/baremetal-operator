package util

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/apply"
	appsv1 "k8s.io/api/apps/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	certPath = "/usr/local/share/ca-certificates/cert.pem"

	// ClusterIDEnvVar ClusterID of the cluster
	ClusterIDEnvVar = "CLUSTER_ID"
	// ProjectIDEnvVar ProjectID of the cluster
	ProjectIDEnvVar = "PROJECT_ID"
	// CloudProviderTypeEnvVar Cloud Provider type of the cluster
	CloudProviderTypeEnvVar = "CLOUD_PROVIDER_TYPE"

	healthCheckSleepEnvVar  = "HEALTHCHECK_INTERVAL_SECS"
	healthCheckSleepDefault = "150"

	// DisableSunpikeEnvVar Disable sunpike sync
	DisableSunpikeEnvVar = "DISABLE_SUNPIKE_SYNC"
	// DisableSunpikeVal value used to disable sync
	DisableSunpikeVal = "true"

	// DisableWatchEnvVar Disable watch
	DisableWatchEnvVar = "DISABLE_WATCH"
	// DisableWatchVal value used to disable watch
	DisableWatchVal = "true"

	watchSleepEnvVar  = "WATCH_SLEEP_SECS"
	watchSleepDefault = "300"

	maxSyncErrorCountEnvVar  = "MAX_SYNC_ERR_COUNT"
	maxSyncErrorCountDefault = "10"

	// DuFqdnEnvVar DU FQDN
	DuFqdnEnvVar = "DU_FQDN"

	unitTestEnvVar = "UNIT_TEST"
)

var templateDir, createDir string

func init() {
	if os.Getenv(unitTestEnvVar) == "" {
		templateDir = "/addon_templates/"
	} else {
		templateDir = "../../addon_templates/"
	}
	createDir = templateDir + "create/"
}

// Labels is used to add lables to a resource
type Labels struct {
	Key   string
	Value string
}

// CheckEnvVarsOnBootup checks all env variables and their format on startup
func CheckEnvVarsOnBootup() {
	var err error

	if err = getEnvUUID(ClusterIDEnvVar); err != nil {
		panic(err)
	}

	if err = getEnvUUID(ProjectIDEnvVar); err != nil {
		panic(err)
	}

	if _, err = getEnvInt(healthCheckSleepEnvVar, healthCheckSleepDefault); err != nil {
		panic(err)
	}

	if _, err = getEnvInt(watchSleepEnvVar, watchSleepDefault); err != nil {
		panic(err)
	}

	if _, err = getEnvInt(maxSyncErrorCountEnvVar, maxSyncErrorCountDefault); err != nil {
		panic(err)
	}

	if duFqdn := os.Getenv(DuFqdnEnvVar); duFqdn == "" {
		panic(fmt.Sprintf("%s not defined as env variable", DuFqdnEnvVar))
	}
}

// GetHealthCheckSleep gets sleep val for health check loop
func GetHealthCheckSleep() int {
	val, _ := getEnvInt(healthCheckSleepEnvVar, healthCheckSleepDefault)
	return val
}

// GetWatchSleep gets sleep val for watch loop
func GetWatchSleep() int {
	val, _ := getEnvInt(watchSleepEnvVar, watchSleepDefault)
	return val
}

// GetSyncErrorCount gets max sync error count
func GetSyncErrorCount() int {
	val, _ := getEnvInt(maxSyncErrorCountEnvVar, maxSyncErrorCountDefault)
	return val
}

func getEnvInt(env, def string) (int, error) {
	value, exists := os.LookupEnv(env)
	if !exists {
		value = def
	}

	return strconv.Atoi(value)
}

func getEnvUUID(env string) error {
	value, exists := os.LookupEnv(env)
	if !exists {
		return fmt.Errorf("%s not defined as env variable", env)
	}

	if !isValidUUID(value) {
		return fmt.Errorf("Invalid UUID: %s", env)
	}

	return nil
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

//GetRegistry gets the override registry value or the default one
func GetRegistry(envVar, defaultValue string) string {
	registry := os.Getenv(envVar)
	if registry == "" {
		registry = defaultValue
	}
	return registry
}

//UpdateCACerts updates ca-cert of the DU
func UpdateCACerts() error {

	_, err := os.Stat(certPath)
	if os.IsNotExist(err) {
		log.Errorf("certPath not found ignoring ca certs: %s", err)
		return nil
	}

	cmd := exec.Command("update-ca-certificates")
	var errb, outb bytes.Buffer
	cmd.Stderr = &errb
	cmd.Stdout = &outb

	cmd.Start()

	if err := cmd.Wait(); err != nil {
		output := errb.String() + outb.String()
		log.Errorf("Error updating ca certs %s", output)
		return err
	}

	return nil
}

//EnsureDirStructure ensures expected dir structure
func EnsureDirStructure(name, version string) (string, string, error) {

	versionDir := name + "/" + version
	inputPath := filepath.Join(templateDir, versionDir)
	outputPath := filepath.Join(createDir, versionDir)

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
		if strings.HasPrefix(p.Name, "base64Enc") {
			b, err := base64.StdEncoding.DecodeString(p.Value)
			if err != nil {
				return nil, err
			}
			p.Value = string(b)
			log.Debugf("Decoded param %s:%s", p.Name, p.Value)
		}
		manifestParams[p.Name] = p.Value
	}

	return manifestParams, nil
}

//WriteConfigToTemplate writes templatized yaml to output dir
func WriteConfigToTemplate(inputPath, outputPath, fileName string, params map[string]interface{}) error {
	t, err := template.New(fileName).Funcs(sprig.TxtFuncMap()).ParseFiles(inputPath)
	if err != nil {
		return err
	}

	if err := renderTemplateToFile(params, t, outputPath); err != nil {
		return err
	}

	return nil
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
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return secret, nil
}

//CreateSecret creates a secret
func CreateSecret(ns, name, key string, data []byte, c client.Client) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Data: map[string][]byte{
			key: data,
		},
	}

	err := c.Create(context.Background(), secret)
	if err != nil {
		return err
	}

	return nil
}

//DeleteSecret deletes a secret
func DeleteSecret(ns, name string, c client.Client) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}

	err := c.Delete(context.Background(), secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	return nil
}

//GetConfigMap gets a configmap
func GetConfigMap(ns, name string, c client.Client) (*corev1.ConfigMap, error) {
	cfgMap := &corev1.ConfigMap{}

	err := c.Get(context.Background(), client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, cfgMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return cfgMap, nil
}

//DeleteConfigMap deletes a configmap
func DeleteConfigMap(ns, name string, c client.Client) error {
	cfgMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}

	err := c.Delete(context.Background(), cfgMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	return nil
}

//CreateConfigMap creates a configmap
func CreateConfigMap(ns, name, key string, data []byte, c client.Client) error {
	cfgMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Data: map[string]string{
			key: string(data),
		},
	}

	err := c.Create(context.Background(), cfgMap)
	if err != nil {
		return err
	}

	return nil
}

//CheckClusterUpgrading check if cluster is in upgrading mode
func CheckClusterUpgrading(c client.Client) (bool, error) {

	cm, err := GetConfigMap("default", "pmk", c)
	if err != nil {
		log.Errorf("Failed to get configmap pmk: %s", err)
		return false, err
	}

	if cm != nil {
		v, e := cm.Data["upgrading"]
		if e && v == "true" {
			return true, nil
		}
	}

	return false, nil
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

//GetDeployment gets a Deployment
func GetDeployment(ns, name string, c client.Client) (*appsv1.Deployment, error) {
	d := &appsv1.Deployment{}

	err := c.Get(context.Background(), client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, d)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return d, nil
}

//DeleteDeployment deletes a Deployment
func DeleteDeployment(ns, name string, c client.Client) error {
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Namespace: ns,
		Name:      name,
	}}

	err := c.Delete(context.Background(), d)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	return nil
}

//GetDaemonset gets a Daemonset
func GetDaemonset(ns, name string, c client.Client) (*appsv1.DaemonSet, error) {
	d := &appsv1.DaemonSet{}

	err := c.Get(context.Background(), client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, d)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return d, nil
}

//DeleteDaemonset deletes a Daemonset
func DeleteDaemonset(ns, name string, c client.Client) error {

	d := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: ns,
		Name:      name,
	}}
	err := c.Delete(context.Background(), d)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	return nil
}

//GetStatefulSet gets a StatefulSet
func GetStatefulSet(ns, name string, c client.Client) (*appsv1.StatefulSet, error) {
	s := &appsv1.StatefulSet{}

	err := c.Get(context.Background(), client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, s)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return s, nil
}

// CreateNsIfNeeded creates ns if not present
func CreateNsIfNeeded(name string, labels []Labels, c client.Client) error {
	ctx := context.Background()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := c.Get(ctx, client.ObjectKey{
		Name: name,
	}, ns)
	if err != nil {
		if apierrors.IsNotFound(err) {
			for _, l := range labels {
				ns.ObjectMeta.Labels = map[string]string{
					l.Key: l.Value,
				}
			}

			return c.Create(ctx, ns)
		}

		log.Error(err, "while querying namespace")
		return err
	}

	return nil
}

//DeleteObject deletes an unstructured object
func DeleteObject(ns, name, kind, apiVersion string, c client.Client) error {
	obj := &uns.Unstructured{}
	//obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetNamespace(ns)
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)

	if err := c.Delete(context.Background(), obj); err != nil {
		log.Error(err, " deleting object")
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	log.Infof("Deleted %s %s/%s successfully", kind, ns, name)
	return nil
}
