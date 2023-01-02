//Package common types
package common

type Manifest []struct {
	Config   string
	RepoTags []string
	Layers   []string
}

type ImageConfigDetails struct {
	Env []string `json:"Env"`
	Cmd []string `json:"Cmd"`
}

type ImageConfig struct {
	Config ImageConfigDetails `json:"config"`
}
