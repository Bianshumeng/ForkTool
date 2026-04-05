package model

type RepoConfig struct {
	Path      string `json:"path" mapstructure:"path" yaml:"path"`
	Kind      string `json:"kind" mapstructure:"kind" yaml:"kind"`
	RemoteURL string `json:"remoteUrl,omitempty" mapstructure:"remoteUrl" yaml:"remoteUrl,omitempty"`
	Tag       string `json:"tag,omitempty" mapstructure:"tag" yaml:"tag,omitempty"`
	Commit    string `json:"commit,omitempty" mapstructure:"commit" yaml:"commit,omitempty"`
}

type ManifestRef struct {
	Path string `json:"path" mapstructure:"path" yaml:"path"`
}

type DecisionFileRef struct {
	Path string `json:"path" mapstructure:"path" yaml:"path"`
}

type OutputConfig struct {
	Dir     string   `json:"dir" mapstructure:"dir" yaml:"dir"`
	Formats []string `json:"formats" mapstructure:"formats" yaml:"formats"`
}

type WorkspaceConfig struct {
	ToolVersion  string          `json:"toolVersion" mapstructure:"toolVersion" yaml:"toolVersion"`
	LocalRepo    RepoConfig      `json:"localRepo" mapstructure:"localRepo" yaml:"localRepo"`
	OfficialRepo RepoConfig      `json:"officialRepo" mapstructure:"officialRepo" yaml:"officialRepo"`
	Manifest     ManifestRef     `json:"manifest" mapstructure:"manifest" yaml:"manifest"`
	DecisionFile DecisionFileRef `json:"decisionFile" mapstructure:"decisionFile" yaml:"decisionFile"`
	Output       OutputConfig    `json:"output" mapstructure:"output" yaml:"output"`
}
