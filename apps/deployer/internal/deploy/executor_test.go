package deploy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zhblogs-deployer/internal/config"
)

type fakeRunner struct {
	failOn string
	calls  []string
}

func (runner *fakeRunner) Run(_ context.Context, _ string, _ []string, name string, args ...string) (CommandResult, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	runner.calls = append(runner.calls, command)
	result := CommandResult{Name: name, Command: append([]string{name}, args...), ExitCode: 0}
	if runner.failOn != "" && strings.Contains(command, runner.failOn) {
		result.ExitCode = 1
		return result, errors.New("command failed")
	}
	return result, nil
}

func TestExecutorUpdatesDockerInOrder(t *testing.T) {
	runner := &fakeRunner{}
	executor := NewExecutorWithRunner(testConfig(), runner)
	class := Classification{Decision: DecisionPartial, ServerModules: []string{ModuleWeb, ModuleAPI}}

	_, err := executor.Execute(context.Background(), WebhookPayload{ImageTag: "sha"}, class)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"git -C /repo pull --ff-only origin main",
		"docker compose -f /repo/infra/docker/docker-compose.prod.yml pull web api",
		"docker compose -f /repo/infra/docker/docker-compose.prod.yml up -d api",
		"docker compose -f /repo/infra/docker/docker-compose.prod.yml up -d web",
	}
	if strings.Join(runner.calls, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected calls:\n%s", strings.Join(runner.calls, "\n"))
	}
}

func TestExecutorReturnsDockerFailure(t *testing.T) {
	runner := &fakeRunner{failOn: "pull"}
	executor := NewExecutorWithRunner(testConfig(), runner)
	class := Classification{Decision: DecisionPartial, ServerModules: []string{ModuleWorker}}

	_, err := executor.Execute(context.Background(), WebhookPayload{ImageTag: "sha"}, class)
	if err == nil {
		t.Fatal("expected docker failure")
	}
}

func TestExecutorReturnsSystemdFailure(t *testing.T) {
	runner := &fakeRunner{failOn: "systemctl restart"}
	executor := NewExecutorWithRunner(testConfig(), runner)
	class := Classification{Decision: DecisionPartial, ServerModules: []string{ModuleDeployer}}

	_, err := executor.Execute(context.Background(), WebhookPayload{ImageTag: "sha", NeedsDeployerUpdate: true}, class)
	if err == nil {
		t.Fatal("expected systemd failure")
	}
}

func TestExecutorSelfUpdateWaitsForResume(t *testing.T) {
	runner := &fakeRunner{}
	executor := NewExecutorWithRunner(testConfig(), runner)
	class := Classification{Decision: DecisionPartial, ServerModules: []string{ModuleAPI}}

	_, err := executor.Execute(context.Background(), WebhookPayload{ImageTag: "sha", NeedsDeployerUpdate: true}, class)
	if !errors.Is(err, ErrWaitingResume) {
		t.Fatalf("expected waiting resume, got %v", err)
	}

	expected := []string{
		"git -C /repo pull --ff-only origin main",
		"go build -o /tmp/zhblogs-deployer-next ./",
		"cp /srv/zhblogs/bin/zhblogs-deployer /srv/zhblogs/bin/zhblogs-deployer.prev",
		"install -m 0755 /tmp/zhblogs-deployer-next /srv/zhblogs/bin/zhblogs-deployer",
		"systemctl restart --no-block zhblogs-deployer",
	}
	if strings.Join(runner.calls, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected calls:\n%s", strings.Join(runner.calls, "\n"))
	}
}

func TestExecutorResumeRunsInfraMigrateThenServices(t *testing.T) {
	runner := &fakeRunner{}
	executor := NewExecutorWithRunner(testConfig(), runner)
	class := Classification{Decision: DecisionPartial, ServerModules: []string{ModuleWorker, ModuleAPI}}
	payload := WebhookPayload{
		ImageTag:            "sha",
		NeedsDeployerUpdate: true,
		NeedsInfraSync:      true,
		NeedsDBMigrate:      true,
	}

	_, err := executor.ExecuteResume(context.Background(), payload, class)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"git -C /repo pull --ff-only origin main",
		"test -f /repo/infra/docker/docker-compose.prod.yml",
		"test -f /repo/infra/nginx/zhblogs.conf",
		"sh -c sudo /usr/local/sbin/zhblogs-apply-infra",
		"pnpm env:prod -- pnpm -F @zhblogs/db run migrate",
		"docker compose -f /repo/infra/docker/docker-compose.prod.yml pull worker api",
		"docker compose -f /repo/infra/docker/docker-compose.prod.yml up -d api",
		"docker compose -f /repo/infra/docker/docker-compose.prod.yml up -d worker",
	}
	if strings.Join(runner.calls, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("unexpected calls:\n%s", strings.Join(runner.calls, "\n"))
	}
}

func testConfig() config.Config {
	return config.Config{
		RepoDir:           "/repo",
		ComposeFile:       "/repo/infra/docker/docker-compose.prod.yml",
		SystemdUnit:       "zhblogs-deployer",
		DockerNamespace:   "zhblogs",
		DockerBin:         "docker",
		SystemctlBin:      "systemctl",
		GoBin:             "go",
		BinaryPath:        "/srv/zhblogs/bin/zhblogs-deployer",
		ConfigTargetDir:   "/repo/infra",
		ConfigSyncCommand: "sudo /usr/local/sbin/zhblogs-apply-infra",
	}
}
