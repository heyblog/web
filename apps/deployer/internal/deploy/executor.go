package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"zhblogs-deployer/internal/config"
)

type Runner interface {
	Run(ctx context.Context, cwd string, env []string, name string, args ...string) (CommandResult, error)
}

type Executor struct {
	cfg    config.Config
	runner Runner
}

type OSRunner struct{}

var ErrWaitingResume = errors.New("waiting for deployer restart")

func NewExecutor(cfg config.Config) Executor {
	return Executor{cfg: cfg, runner: OSRunner{}}
}

func NewExecutorWithRunner(cfg config.Config, runner Runner) Executor {
	return Executor{cfg: cfg, runner: runner}
}

func (executor Executor) Execute(ctx context.Context, payload WebhookPayload, class Classification) (ExecutionResult, error) {
	return executor.execute(ctx, payload, class, false)
}

func (executor Executor) ExecuteResume(ctx context.Context, payload WebhookPayload, class Classification) (ExecutionResult, error) {
	return executor.execute(ctx, payload, class, true)
}

func (executor Executor) execute(ctx context.Context, payload WebhookPayload, class Classification, resumed bool) (ExecutionResult, error) {
	result := ExecutionResult{}
	if class.Decision == DecisionSkip && !payload.HasDeployerAction() {
		return result, nil
	}

	if err := executor.run(ctx, &result, nil, "git", "-C", executor.cfg.RepoDir, "pull", "--ff-only", "origin", "main"); err != nil {
		return result, err
	}

	if payload.NeedsEnvManualReview {
		executor.notifyAdmin(ctx, &result, payload)
	}

	if payload.NeedsDeployerUpdate && !resumed {
		if err := executor.updateSelf(ctx, &result); err != nil {
			return result, err
		}
		return result, ErrWaitingResume
	}

	if payload.NeedsInfraSync {
		if err := executor.syncInfra(ctx, &result); err != nil {
			return result, err
		}
	}

	if payload.NeedsDBMigrate {
		if err := executor.migrateDB(ctx, &result); err != nil {
			return result, err
		}
	}

	env := executor.composeEnv(payload)
	if err := executor.updateDockerServices(ctx, &result, env, class.ServerModules); err != nil {
		return result, err
	}

	return result, nil
}

func (executor Executor) Rollback(ctx context.Context, payload WebhookPayload) (ExecutionResult, error) {
	result := ExecutionResult{}
	env := executor.composeEnv(payload)
	err := executor.run(ctx, &result, env, executor.cfg.DockerBin, "compose", "-f", executor.cfg.ComposeFile, "up", "-d", "api", "web", "worker")
	return result, err
}

func (executor Executor) updateDockerServices(ctx context.Context, result *ExecutionResult, env []string, modules []string) error {
	services := servicesForModules(modules)
	if len(services) == 0 {
		return nil
	}

	args := append([]string{"compose", "-f", executor.cfg.ComposeFile, "pull"}, services...)
	if err := executor.run(ctx, result, env, executor.cfg.DockerBin, args...); err != nil {
		return err
	}

	if contains(services, "api") {
		if err := executor.run(ctx, result, env, executor.cfg.DockerBin, "compose", "-f", executor.cfg.ComposeFile, "up", "-d", "api"); err != nil {
			return err
		}
	}

	rest := without(services, "api")
	if len(rest) == 0 {
		return nil
	}

	args = append([]string{"compose", "-f", executor.cfg.ComposeFile, "up", "-d"}, rest...)
	return executor.run(ctx, result, env, executor.cfg.DockerBin, args...)
}

func (executor Executor) updateSelf(ctx context.Context, result *ExecutionResult) error {
	tmpBinary := filepath.Join("/tmp", "zhblogs-deployer-next")
	deployerDir := filepath.Join(executor.cfg.RepoDir, "apps/deployer")
	previousBinary := executor.cfg.BinaryPath + ".prev"

	if err := executor.runInDir(ctx, result, nil, deployerDir, executor.cfg.GoBin, "build", "-o", tmpBinary, "./"); err != nil {
		return err
	}
	if err := executor.run(ctx, result, nil, "cp", executor.cfg.BinaryPath, previousBinary); err != nil {
		return err
	}
	if err := executor.runInDir(ctx, result, nil, deployerDir, "install", "-m", "0755", tmpBinary, executor.cfg.BinaryPath); err != nil {
		_ = executor.run(ctx, result, nil, "cp", previousBinary, executor.cfg.BinaryPath)
		return err
	}
	if err := executor.run(ctx, result, nil, executor.cfg.SystemctlBin, "restart", "--no-block", executor.cfg.SystemdUnit); err != nil {
		_ = executor.run(ctx, result, nil, "cp", previousBinary, executor.cfg.BinaryPath)
		return err
	}
	return nil
}

func (executor Executor) syncInfra(ctx context.Context, result *ExecutionResult) error {
	requiredFiles := []string{
		filepath.Join(executor.cfg.RepoDir, "infra", "docker", "docker-compose.prod.yml"),
		filepath.Join(executor.cfg.RepoDir, "infra", "nginx", "zhblogs.conf"),
	}
	for _, file := range requiredFiles {
		if err := executor.run(ctx, result, nil, "test", "-f", file); err != nil {
			return err
		}
	}
	if executor.cfg.ConfigSyncCommand == "" {
		return nil
	}
	return executor.run(ctx, result, nil, "sh", "-c", executor.cfg.ConfigSyncCommand)
}

func (executor Executor) migrateDB(ctx context.Context, result *ExecutionResult) error {
	return executor.run(ctx, result, nil, "pnpm", "env:prod", "--", "pnpm -F @zhblogs/db run migrate")
}

func (executor Executor) notifyAdmin(ctx context.Context, result *ExecutionResult, payload WebhookPayload) {
	if executor.cfg.AdminNotifyWebhook == "" {
		return
	}
	body, err := json.Marshal(map[string]any{
		"event":         "env_manual_review_required",
		"repository":    payload.Repository,
		"commit_sha":    payload.CommitSHA,
		"workflow_url":  payload.WorkflowRunURL,
		"changed_files": payload.ChangedFiles,
	})
	if err != nil {
		return
	}
	_ = executor.run(ctx, result, nil, "curl", "-fsS", "-X", "POST", "-H", "content-type: application/json", "-d", string(body), executor.cfg.AdminNotifyWebhook)
}

func (executor Executor) run(ctx context.Context, result *ExecutionResult, env []string, name string, args ...string) error {
	return executor.runInDir(ctx, result, env, executor.cfg.RepoDir, name, args...)
}

func (executor Executor) runInDir(ctx context.Context, result *ExecutionResult, env []string, cwd string, name string, args ...string) error {
	commandResult, err := executor.runner.Run(ctx, cwd, env, name, args...)
	result.Commands = append(result.Commands, commandResult)
	return err
}

func (executor Executor) composeEnv(payload WebhookPayload) []string {
	namespace := payload.DockerhubNamespace
	if namespace == "" {
		namespace = executor.cfg.DockerNamespace
	}
	return []string{
		"DOCKERHUB_NAMESPACE=" + namespace,
		"IMAGE_TAG=" + payload.ImageTag,
	}
}

func (OSRunner) Run(ctx context.Context, cwd string, env []string, name string, args ...string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	cmd.Env = append(cmd.Environ(), env...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return CommandResult{
		Name:     name,
		Command:  append([]string{name}, args...),
		ExitCode: exitCode,
		Output:   trimOutput(output.String()),
	}, err
}

func servicesForModules(modules []string) []string {
	if containsModule(modules, ModuleAll) {
		return []string{"api", "web", "worker"}
	}

	services := make([]string, 0, 3)
	for _, module := range modules {
		switch module {
		case ModuleAPI:
			services = append(services, "api")
		case ModuleWeb:
			services = append(services, "web")
		case ModuleWorker:
			services = append(services, "worker")
		}
	}
	return uniqueStrings(services)
}

func containsModule(modules []string, module string) bool {
	return contains(modules, module) || contains(modules, ModuleAll)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func without(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func trimOutput(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4000 {
		return value
	}
	return fmt.Sprintf("%s\n...truncated...", value[:4000])
}
