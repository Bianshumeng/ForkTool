package model

type DecisionFile struct {
	Version   int            `json:"version" yaml:"version"`
	Decisions []DecisionHint `json:"decisions" yaml:"decisions"`
}
