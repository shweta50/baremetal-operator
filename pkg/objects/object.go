package objects

const (
	//TemplateDir points to path for addon yamls
	TemplateDir = "/addon_templates/"
	//CreateDir path to yamls to create addons
	CreateDir = TemplateDir + "create/"
	//DeleteDir path to yamls to delete addons
	DeleteDir = TemplateDir + "delete/"
	//ManifestFile lists all addons available
	ManifestFile = "manifest.json"
)

//AddonState lists of Addons on the cluster
type AddonState struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Phase   string `json:"phase,omitempty"`
}
