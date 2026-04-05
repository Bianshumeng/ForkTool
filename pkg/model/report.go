package model

import "time"

type NodeKind string

const (
	NodeKindRoute       NodeKind = "route"
	NodeKindHandler     NodeKind = "handler"
	NodeKindService     NodeKind = "service"
	NodeKindHelper      NodeKind = "helper"
	NodeKindTransformer NodeKind = "transformer"
	NodeKindTest        NodeKind = "test"
	NodeKindConfig      NodeKind = "config"
	NodeKindMigration   NodeKind = "migration"
	NodeKindPage        NodeKind = "page"
	NodeKindNav         NodeKind = "nav"
	NodeKindAPI         NodeKind = "api"
	NodeKindDTO         NodeKind = "dto"
	NodeKindStore       NodeKind = "store"
	NodeKindDoc         NodeKind = "doc"
)

type SourceRange struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

type NodeRelation struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

type ChainNode struct {
	Kind       NodeKind       `json:"kind"`
	Language   string         `json:"language"`
	FilePath   string         `json:"filePath,omitempty"`
	SymbolName string         `json:"symbolName,omitempty"`
	Range      SourceRange    `json:"range,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Relations  []NodeRelation `json:"relations,omitempty"`
}

type DecisionHint struct {
	FeatureID string `json:"featureId"`
	Scope     string `json:"scope"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason"`
	Source    string `json:"source"`
}

type FeatureChain struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	RiskLevel     string         `json:"riskLevel"`
	Languages     []string       `json:"languages"`
	LocalNodes    []ChainNode    `json:"localNodes"`
	OfficialNodes []ChainNode    `json:"officialNodes"`
	SemanticRules []string       `json:"semanticRules"`
	DecisionHints []DecisionHint `json:"decisionHints,omitempty"`
}

type EvidenceRef struct {
	RepoSide   string `json:"repoSide,omitempty"`
	FilePath   string `json:"filePath,omitempty"`
	SymbolName string `json:"symbolName,omitempty"`
	Note       string `json:"note,omitempty"`
}

type SemanticDiff struct {
	FeatureID         string        `json:"featureId"`
	Severity          string        `json:"severity"`
	Category          string        `json:"category"`
	Title             string        `json:"title"`
	Description       string        `json:"description,omitempty"`
	Evidence          []EvidenceRef `json:"evidence,omitempty"`
	DecisionTag       string        `json:"decisionTag"`
	RecommendedAction string        `json:"recommendedAction"`
}

type RepoSnapshot struct {
	Path      string `json:"path,omitempty"`
	Kind      string `json:"kind,omitempty"`
	RemoteURL string `json:"remoteUrl,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Commit    string `json:"commit,omitempty"`
}

type AuditSummary struct {
	FeaturesScanned   int      `json:"featuresScanned"`
	CriticalFindings  int      `json:"criticalFindings"`
	HighFindings      int      `json:"highFindings"`
	MediumFindings    int      `json:"mediumFindings"`
	LowFindings       int      `json:"lowFindings"`
	RecommendedAction string   `json:"recommendedAction,omitempty"`
	Notes             []string `json:"notes,omitempty"`
}

type FeatureReport struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	RiskLevel     string         `json:"riskLevel"`
	Status        string         `json:"status"`
	SemanticRules []string       `json:"semanticRules,omitempty"`
	LocalNodes    []ChainNode    `json:"localNodes,omitempty"`
	OfficialNodes []ChainNode    `json:"officialNodes,omitempty"`
	Findings      []SemanticDiff `json:"findings,omitempty"`
	Notes         []string       `json:"notes,omitempty"`
}

type AuditReport struct {
	RunID           string          `json:"runId"`
	GeneratedAt     time.Time       `json:"generatedAt"`
	LocalRepo       RepoSnapshot    `json:"localRepo"`
	OfficialRepo    RepoSnapshot    `json:"officialRepo"`
	ManifestVersion int             `json:"manifestVersion"`
	Summary         AuditSummary    `json:"summary"`
	Features        []FeatureReport `json:"features"`
}

type BaselineCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type BaselineVerificationResult struct {
	Official          RepoSnapshot    `json:"official"`
	RemoteName        string          `json:"remoteName"`
	ExpectedRemoteURL string          `json:"expectedRemoteUrl,omitempty"`
	ActualRemoteURL   string          `json:"actualRemoteUrl,omitempty"`
	ExpectedTag       string          `json:"expectedTag,omitempty"`
	ExpectedCommit    string          `json:"expectedCommit,omitempty"`
	ResolvedTagCommit string          `json:"resolvedTagCommit,omitempty"`
	ResolvedCommit    string          `json:"resolvedCommit,omitempty"`
	Valid             bool            `json:"valid"`
	Checks            []BaselineCheck `json:"checks"`
	Errors            []string        `json:"errors,omitempty"`
}

type ManifestValidationResult struct {
	Path         string   `json:"path"`
	RepoKind     string   `json:"repoKind,omitempty"`
	Version      int      `json:"version,omitempty"`
	FeatureCount int      `json:"featureCount,omitempty"`
	Valid        bool     `json:"valid"`
	Errors       []string `json:"errors,omitempty"`
}

type RunContext struct {
	RunID            string                      `json:"runId"`
	GeneratedAt      time.Time                   `json:"generatedAt"`
	ToolVersion      string                      `json:"toolVersion"`
	ManifestPath     string                      `json:"manifestPath,omitempty"`
	DecisionFilePath string                      `json:"decisionFilePath,omitempty"`
	OutputDir        string                      `json:"outputDir"`
	LocalRepo        RepoSnapshot                `json:"localRepo"`
	OfficialRepo     RepoSnapshot                `json:"officialRepo"`
	Baseline         *BaselineVerificationResult `json:"baseline,omitempty"`
}

type ScanFeatureResult struct {
	FeatureID      string   `json:"featureId"`
	RunID          string   `json:"runId"`
	OutputDir      string   `json:"outputDir"`
	ContextPath    string   `json:"contextPath"`
	ReportFiles    []string `json:"reportFiles"`
	BaselineStatus string   `json:"baselineStatus"`
	DiscoveryMode  string   `json:"discoveryMode"`
}
