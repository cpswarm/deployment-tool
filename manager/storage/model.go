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
	if o.Build == nil && o.Deploy == nil {
		return fmt.Errorf("neither build nor deploy are defined")
	}

	// validate build
	if o.Build != nil && len(o.Build.Commands)+len(o.Build.Artifacts)+len(o.Build.Host) > 0 {
		if len(o.Build.Commands) == 0 {
			return fmt.Errorf("build.commands empty")
		}
		if len(o.Build.Artifacts) == 0 {
			return fmt.Errorf("build.artifacts empty")
		}
		if o.Build.Host == "" {
			return fmt.Errorf("build.host not given")
		}

		for _, path := range o.Build.Artifacts {
			if strings.HasPrefix(path, "/") {
				return fmt.Errorf("path in build.artifacts should be relative to source. Given path is absolute: %s", path)
			}
		}
	}

	// validate deploy
	if o.Deploy != nil && len(o.Deploy.Install.Commands)+len(o.Deploy.Run.Commands)+len(o.Deploy.Target.IDs)+len(o.Deploy.Target.Tags) > 0 {
		if len(o.Deploy.Target.IDs)+len(o.Deploy.Target.Tags) == 0 {
			return fmt.Errorf("both deploy.target.ids and deploy.target.ids are empty")
		}
		if len(o.Deploy.Install.Commands)+len(o.Deploy.Run.Commands) == 0 {
			return fmt.Errorf("both deploy.install.commands and deploy.run.commands are empty")
		}
	}

	return nil
}

//
// TARGET
//
type Target struct {
	model.TargetBase
	CreatedAt    model.UnixTimeType `json:"createdAt,omitempty"`
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
