package token

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"bytes"
	"errors"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	addonerr "github.com/platform9/pf9-addon-operator/pkg/errors"
)

const (
	defaultCfgFile  = "etc/addon/config.yaml"
	kubeCfgTemplate = "etc/addon/keystone.kubeconfig.template"
)

//	KubeCfg    []byte

// KsCreds represents keystone credentials
type KsCreds struct {
	UserName string
	Password string
	AuthURL  string
	Project  string
	Domain   string
}

// Qbert represents the qbert metadata
type Qbert struct {
	url string
	ks  KsCreds
}

type (
	keystoneAuthRequest struct {
		Auth struct {
			Identity struct {
				Methods  []string        `json:"methods"`
				Password *passwordMethod `json:"password,omitempty"`
			} `json:"identity"`
			Scope struct {
				Project struct {
					Name   string `json:"name,omitempty"`
					ID     string `json:"id,omitempty"`
					Domain struct {
						ID string `json:"id"`
					} `json:"domain"`
				} `json:"project"`
			} `json:"scope"`
		} `json:"auth"`
	}

	passwordMethod struct {
		User struct {
			Name   string `json:"name"`
			Domain struct {
				Name string `json:"name"`
			} `json:"domain"`
			Password string `json:"password"`
		} `json:"user"`
	}
)

type users struct {
	Users []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"users"`
}

type roles struct {
	Roles []struct {
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
	} `json:"roles"`
}

// keep valid token cache by project
type tokenEntry struct {
	token   string
	expires time.Time
}

var tokenCache map[string]tokenEntry
var qbert *Qbert

func init() {
	tokenCache = make(map[string]tokenEntry, 5)

	//Warm up the token cache for "services"
	//qbert = GetConfig()
}

// Adds keystone user to a keystone project
func addUserToProject(authURL, token, userid, projectid, roleid string) error {
	url := fmt.Sprintf("%s/projects/%s/users/%s/roles/%s", authURL, projectid, userid, roleid)
	req, err := http.NewRequest("PUT", url, nil)

	if err != nil {
		return err
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	cl := &http.Client{}
	resp, err := cl.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected response from keystone: (%d) %s", resp.StatusCode, resp.Body)
	}

	return nil
}

func getUserID(authURL, token, username string) (string, error) {
	url := fmt.Sprintf("%s/users?name=%s", authURL, username)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	cl := &http.Client{}
	resp, err := cl.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var userInfos users
	if err = json.Unmarshal(respData, &userInfos); err != nil {
		return "", err
	}

	return userInfos.Users[0].ID, nil
}

func getRoleID(authURL, token, rolename string) (string, error) {
	url := fmt.Sprintf("%s/roles?name=%s", authURL, rolename)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("Content-Type", "application/json")

	cl := &http.Client{}
	resp, err := cl.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var roleInfos roles
	if err = json.Unmarshal(respData, &roleInfos); err != nil {
		return "", err
	}

	return roleInfos.Roles[0].ID, nil
}

//IsValidUUID check is the passed string is a valid UUID
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// GetKsToken returns keystone token
func GetKsToken(project string) (string, error) {
	// check if token is cached and validate expiry
	var token string
	e, ok := tokenCache[project]
	if ok && e.expires.After(time.Now().Add(2*time.Minute)) {
		return e.token, nil
	}

	token, err := getAndCacheToken(&qbert.ks, project)
	if err == nil {
		log.Infof("Created new token for project: %s", project)
		return token, nil
	}

	log.Errorf("Failed to create new token for project: %s", project)

	if !addonerr.IsNotAuthorized(err) {
		log.Errorf("Appbert not authorized to create new token for project: %s", project)
		return "", err
	}

	// Associate appbert as admin of this project
	st, ok := tokenCache["services"]
	if !ok {
		log.Errorf("No service token found for role association for project: %s", project)
		return "", errors.New("no service token for role association")
	}

	userID, err := getUserID(qbert.ks.AuthURL, st.token, qbert.ks.UserName)
	if err != nil {
		log.Errorf("Failed to get user id for %s for project: %s", qbert.ks.UserName, project)
		return "", err
	}

	roleID, err := getRoleID(qbert.ks.AuthURL, st.token, "admin")
	if err != nil {
		log.Errorf("Failed to get role id for admin for project: %s", project)
		return "", err
	}

	err = addUserToProject(qbert.ks.AuthURL, st.token, userID, project, roleID)
	if err != nil {
		log.Errorf("Failed to add keystone user to project: %s", project)
		return "", err
	}

	log.Infof("Creating new token for project: %s", project)
	// retry token
	return getAndCacheToken(&qbert.ks, project)
}

func getAndCacheToken(cred *KsCreds, project string) (string, error) {
	token, expires, err := getNewToken(cred, project)
	if err != nil {
		return "", err
	}
	tokenCache[project] = tokenEntry{
		token:   token,
		expires: expires,
	}
	return tokenCache[project].token, nil
}

func getNewToken(cred *KsCreds, project string) (token string, expires time.Time, err error) {
	req := keystoneAuthRequest{}
	req.Auth.Identity.Methods = []string{"password"}
	req.Auth.Identity.Password = &passwordMethod{}
	req.Auth.Identity.Password.User.Name = cred.UserName
	req.Auth.Identity.Password.User.Password = cred.Password
	req.Auth.Identity.Password.User.Domain.Name = cred.Domain
	if IsValidUUID(project) {
		req.Auth.Scope.Project.ID = project
	} else {
		req.Auth.Scope.Project.Name = project
	}
	req.Auth.Scope.Project.Domain.ID = cred.Domain

	b, err := json.Marshal(req)
	if err != nil {
		return "", time.Time{}, err
	}

	url := fmt.Sprintf("%s/auth/tokens?nocatalog=1", cred.AuthURL)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))

	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", time.Time{}, addonerr.NotAuthorized()
	}

	if resp.StatusCode != http.StatusCreated {
		return "", time.Time{}, fmt.Errorf("Unexpected keystone response. Code: %d", resp.StatusCode)
	}

	var result struct {
		Token struct {
			Project struct {
				ID string `json:"id"`
			} `json:"project"`
			Expires string `json:"expires_at"`
		} `json:"token"`
	}

	b, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", time.Time{}, err
	}

	err = json.Unmarshal(b, &result)
	if err != nil {
		return "", time.Time{}, err
	}

	token = resp.Header.Get("X-Subject-Token")
	t, err := time.Parse(time.RFC3339Nano, result.Token.Expires)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, t, nil
}

//GetSunpikeKubeCfg gets sunpike kubecfg for a specific cluster
func GetSunpikeKubeCfg(token, clusterID, project string) (*rest.Config, error) {

	/*data, err := ioutil.ReadFile(kubeCfgTemplate)
	if err != nil {
		log.Errorf("Failed to read kubecfg template: %s", err)
		return nil, err
	}

	buf := strings.Replace(string(data), "__DU_QBERT_FQDN__", qbert.url, 1)
	buf = strings.Replace(buf, "__KEYSTONE_TOKEN__", token, 1)
	buf = strings.Replace(buf, "__PROJECT_ID__", project, 1)

	kubeCfgPath := clusterID + ".cfg"

	err = ioutil.WriteFile(kubeCfgPath, []byte(buf), 0600)
	if err != nil {
		log.Errorf("Failed to get write kubecfg: %s", err)
		return nil, err
	}*/
	kubeCfgPath := "85257b24-da0d-4ead-83d2-113241cf2a05.cfg"
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeCfgPath)
	if err != nil {
		return nil, err
	}

	return cfg, err
}

// GetConfig reads config file
func GetConfig() *Qbert {
	q := &Qbert{}

	viper.SetConfigFile(defaultCfgFile)
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	q.url = viper.GetString("qbert.url")
	q.ks.AuthURL = viper.GetString("keystone.url")
	q.ks.Domain = viper.GetString("keystone.domain")
	q.ks.Password = viper.GetString("keystone.password")
	q.ks.Project = viper.GetString("keystone.project")
	q.ks.UserName = viper.GetString("keystone.user")

	qbert = q
	_, err := GetKsToken(q.ks.Project)
	if err != nil {
		panic(err)
	}

	return q
}

//GetQbert returns the global qbert
func GetQbert() *Qbert {
	return qbert
}
