package deploy

import "testing"

func testRules() Rules {
	return Rules{
		FullUpdatePaths: []string{"packages/**", "pnpm-lock.yaml"},
		IgnoredPaths:    []string{"design/**"},
		Modules: []ModuleRule{
			{Module: ModuleAPI, Paths: []string{"apps/api/**"}},
			{Module: ModuleWeb, Paths: []string{"apps/web/**"}},
			{Module: ModuleWorker, Paths: []string{"apps/worker/**"}},
			{Module: ModuleDeployer, Paths: []string{"apps/deployer/**"}},
			{Module: ModuleCloudflare, Paths: []string{"apps/cloudflare/**"}},
		},
	}
}

func TestClassifyFullUpdate(t *testing.T) {
	class := Classify([]string{"packages/db/src/schema/deployments.ts"}, testRules())
	if class.Decision != DecisionFull {
		t.Fatalf("expected full update, got %s", class.Decision)
	}
	if len(class.Modules) != 1 || class.Modules[0] != ModuleAll {
		t.Fatalf("expected ALL module, got %#v", class.Modules)
	}
}

func TestClassifyPartialUpdate(t *testing.T) {
	class := Classify([]string{"apps/api/index.ts", "apps/worker/main.go"}, testRules())
	if class.Decision != DecisionPartial {
		t.Fatalf("expected partial update, got %s", class.Decision)
	}
	if len(class.ServerModules) != 2 {
		t.Fatalf("expected two server modules, got %#v", class.ServerModules)
	}
}

func TestClassifyNonServerUpdate(t *testing.T) {
	class := Classify([]string{"apps/cloudflare/src/index.ts"}, testRules())
	if class.Decision != DecisionNonServer {
		t.Fatalf("expected non-server update, got %s", class.Decision)
	}
	if len(class.NonServerModules) != 1 || class.NonServerModules[0] != ModuleCloudflare {
		t.Fatalf("expected CLOUDFLARE module, got %#v", class.NonServerModules)
	}
}

func TestClassifySkip(t *testing.T) {
	class := Classify([]string{"design/deploy.md"}, testRules())
	if class.Decision != DecisionSkip {
		t.Fatalf("expected skip, got %s", class.Decision)
	}
	if len(class.ChangedFiles) != 0 {
		t.Fatalf("expected ignored files to be removed, got %#v", class.ChangedFiles)
	}
}

func TestClassifyFiltersIgnoredFiles(t *testing.T) {
	class := Classify([]string{"design/deploy.md", "apps/api/index.ts"}, testRules())
	if class.Decision != DecisionPartial {
		t.Fatalf("expected partial update, got %s", class.Decision)
	}
	if len(class.ChangedFiles) != 1 || class.ChangedFiles[0] != "apps/api/index.ts" {
		t.Fatalf("expected only deployable file, got %#v", class.ChangedFiles)
	}
}
