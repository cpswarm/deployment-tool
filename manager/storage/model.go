package storage

import (
	"fmt"
	"strings"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
	"code.linksmart.eu/dt/deployment-tool/manager/source"
)

//
// ORDER
//
type Order struct {
	model.Header `yaml:",inline"`
	Source       *source.Source `json:"source,omitempty"`
	Build        *build         `json:"build"`
	Deploy       *deploy        `json:"deploy"`
}

type build struct {
	model.Build `yaml:",inline"`
	Host        string `json:"host"`
}

type deploy struct {
	model.Deploy `yaml:",inline"`
	Target       struct {
		IDs  []string `json:"ids"`
		Tags []string `json:"tags"`
	} `json:"target"`
	Match Match `json:"match"`
}

type Match struct {
	IDs  []string `json:"ids"` // ids of targets not covered by tags
	Tags []string `json:"tags"`
	List []string `json:"list"` // all ids
}

func (o Order) Validate() error {
	// validate build
	if o.Build != nil {
		if o.Build.Host == "" {
			return fmt.Errorf("build host not given")
		}
		if len(o.Build.Commands) == 0 {
			return fmt.Errorf("build has no commands")
		}
		for _, path := range o.Build.Artifacts {
			if strings.HasPrefix(path, "/") {
				return fmt.Errorf("path to artifact should be relative to source. Given path is absolute: %s", path)
			}
		}
	}

	// validate deploy
	if o.Deploy != nil {
		if len(o.Deploy.Target.IDs)+len(o.Deploy.Target.Tags) == 0 {
			return fmt.Errorf("deploy.target contains no ids and tags")
		}
		if len(o.Deploy.Install.Commands)+len(o.Deploy.Run.Commands) == 0 {
			return fmt.Errorf("deploy has no install and run commands")
		}
	}

	return nil
}

//
// TARGET
//
type Target struct {
	model.TargetBase
	UpdatedAt    model.UnixTimeType `json:"updatedAt,omitempty"`
	LogRequestAt model.UnixTimeType `json:"logRequestAt,omitempty"`
}

//
// LOG
//
type Log struct {
	model.Log
	Target string `json:"target,omitempty"`
}
