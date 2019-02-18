package model

import "fmt"

const (
	// Request types (should not contain PrefixSeparator or TopicSeperator chars)
	RequestTargetAll = "ALL"
	RequestTargetID  = "ID"
	RequestTargetTag = "TAG"
	// Stage types
	StageBuild    = "build"
	StageTransfer = "transfer"
	StageInstall  = "install"
	StageRun      = "run"
	// Topic consts
	PrefixSeparator = "-"
	// Task types
	TaskTypeBuild  = 1
	TaskTypeDeploy = 2
)

type Build struct {
	Commands  []string `json:"commands"`
	Artifacts []string `json:"artifacts"`
}

type Deploy struct {
	Install struct {
		Commands []string `json:"commands"`
	} `json:"install"`
	Run struct {
		Commands    []string `json:"commands"`
		AutoRestart bool     `json:"autoRestart"`
	} `json:"run"`
}

// Header contains information that is common among task related structs
type Header struct {
	ID      string `json:"id"`
	Debug   bool   `json:"debug,omitempty"`
	Created int64  `json:"createdAt"`
	//BuildType bool   `json:"b,omitempty"`
}

// Announcement carries information about a task
type Announcement struct {
	Header
	Size int   `json:"s"`
	Type uint8 `json:"b,omitempty"`
}

// Task is a struct with all the information for deployment on a target
type Task struct {
	Header
	//Stages    Stages `json:"stages"`
	Build     *Build  `json:"bl,omitempty"`
	Deploy    *Deploy `json:"de,omitempty"`
	Artifacts []byte  `json:"ar,omitempty"`
}

func (t *Task) Validate() error {
	if t.Build != nil && t.Deploy != nil {
		return fmt.Errorf("task contains both build and deploy stages")
	}
	return nil
}

type LogRequest struct {
	IfModifiedSince UnixTimeType
}

// RequestWrapper is the struct of messages sent to request topics
type RequestWrapper struct {
	Announcement *Announcement `json:"a,omitempty"`
	LogRequest   *LogRequest   `json:"l,omitempty"`
	Command      *string       `json:"c,omitempty"`
}

func FormatTopicID(id string) string {
	return RequestTargetID + PrefixSeparator + id
}

func FormatTopicTag(tag string) string {
	return RequestTargetTag + PrefixSeparator + tag
}
