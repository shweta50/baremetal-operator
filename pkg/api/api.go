package api

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	agentv1 "github.com/platform9/pf9-addon-operator/api/v1"
	"github.com/platform9/pf9-addon-operator/pkg/k8s"
	"github.com/platform9/pf9-addon-operator/pkg/objects"
	"github.com/platform9/pf9-addon-operator/pkg/util"
)

// Status fetches list of installed addons
func Status(resp http.ResponseWriter, req *http.Request) {

	scheme := runtime.NewScheme()
	_ = agentv1.AddToScheme(scheme)

	cl, err := client.New(config.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		log.Error(err, "Failed to create kube client")
		return
	}

	w, err := k8s.New("k8s", cl)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		log.Error(err, "while processing package")
		return
	}

	currState, err := w.ListAddons()
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		log.Error(err, "unable to list addon")
		return
	}

	data, err := json.MarshalIndent(currState, "", "   ")
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		log.Error(err, "unable to unmarshal response")
		return
	}

	resp.WriteHeader(http.StatusOK)
	resp.Header().Add("Content-Type", "application/json")
	if _, err = resp.Write(data); err != nil {
		log.Error(err, "while responding over http")
	}
}

// AvailableAddons fetches list of available addons that can be installed
func AvailableAddons(resp http.ResponseWriter, req *http.Request) {

	var currState []objects.AddonState

	manifestFilePath := objects.TemplateDir + "/" + objects.ManifestFile
	addonState, err := util.ReadManifestFile(manifestFilePath)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		log.Error(err, "unable to read manifest file")
		return
	}

	for _, a := range addonState {
		currState = append(currState, a)
	}

	data, err := json.MarshalIndent(currState, "", "   ")
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		log.Error(err, "unable to unmarshal response")
		return
	}

	resp.WriteHeader(http.StatusOK)
	resp.Header().Add("Content-Type", "application/json")
	if _, err = resp.Write(data); err != nil {
		log.Error(err, "while responding over http")
	}
}
