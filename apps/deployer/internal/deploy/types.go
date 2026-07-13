package deploy

const (
	DecisionFull      = "FULL"
	DecisionPartial   = "PARTIAL"
	DecisionSkip      = "SKIP"
	DecisionNonServer = "NON_SERVER"

	ModuleAll        = "ALL"
	ModuleAPI        = "API"
	ModuleWeb        = "WEB"
	ModuleWorker     = "WORKER"
	ModuleDeployer   = "DEPLOYER"
	ModuleCloudflare = "CLOUDFLARE"
)

type WebhookPayload struct {
	Event                string   `json:"event"`
	Repository           string   `json:"repository"`
	Ref                  string   `json:"ref"`
	CommitSHA            string   `json:"commit_sha"`
	ImageTag             string   `json:"image_tag"`
	DockerhubNamespace   string   `json:"dockerhub_namespace"`
	WorkflowRunID        string   `json:"workflow_run_id"`
	WorkflowRunURL       string   `json:"workflow_run_url"`
	ChangedFiles         []string `json:"changed_files"`
	NeedsDBMigrate       bool     `json:"needs_db_migrate"`
	NeedsInfraSync       bool     `json:"needs_infra_sync"`
	NeedsDeployerUpdate  bool     `json:"needs_deployer_update"`
	NeedsEnvManualReview bool     `json:"needs_env_manual_review"`
}

type Classification struct {
	Decision         string   `json:"decision"`
	Modules          []string `json:"modules"`
	ServerModules    []string `json:"server_modules"`
	NonServerModules []string `json:"non_server_modules"`
	Reason           string   `json:"reason"`
	ChangedFiles     []string `json:"changed_files"`
}

type CommandResult struct {
	Name     string   `json:"name"`
	Command  []string `json:"command"`
	ExitCode int      `json:"exit_code"`
	Output   string   `json:"output,omitempty"`
}

type ExecutionResult struct {
	Commands []CommandResult `json:"commands"`
}

func (payload WebhookPayload) HasDeployerAction() bool {
	return payload.NeedsDBMigrate ||
		payload.NeedsInfraSync ||
		payload.NeedsDeployerUpdate ||
		payload.NeedsEnvManualReview
}
