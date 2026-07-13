package deploy

import (
	"encoding/json"
	"os"
	"path"
	"strings"
)

type Rules struct {
	FullUpdatePaths []string     `json:"full_update_paths"`
	IgnoredPaths    []string     `json:"ignored_paths"`
	Modules         []ModuleRule `json:"modules"`
}

type ModuleRule struct {
	Module string   `json:"module"`
	Paths  []string `json:"paths"`
}

func LoadRules(file string) (Rules, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Rules{}, err
	}

	var rules Rules
	if err := json.Unmarshal(data, &rules); err != nil {
		return Rules{}, err
	}

	return rules, nil
}

func Classify(files []string, rules Rules) Classification {
	changed := filterIgnoredFiles(normalizeFiles(files), rules.IgnoredPaths)
	if len(changed) == 0 {
		return Classification{Decision: DecisionSkip, Reason: "no changed files"}
	}

	if hasMatch(changed, rules.FullUpdatePaths) {
		return Classification{
			Decision:         DecisionFull,
			Modules:          []string{ModuleAll},
			ServerModules:    []string{ModuleAPI, ModuleWeb, ModuleWorker},
			NonServerModules: matchedNonServerModules(changed, rules),
			Reason:           "full update path changed",
			ChangedFiles:     changed,
		}
	}

	serverModules, nonServerModules := matchedModules(changed, rules)
	if len(serverModules) > 0 {
		return Classification{
			Decision:         DecisionPartial,
			Modules:          appendUnique(serverModules, nonServerModules...),
			ServerModules:    serverModules,
			NonServerModules: nonServerModules,
			Reason:           "server module path changed",
			ChangedFiles:     changed,
		}
	}

	if len(nonServerModules) > 0 {
		return Classification{
			Decision:         DecisionNonServer,
			Modules:          nonServerModules,
			NonServerModules: nonServerModules,
			Reason:           "non-server module path changed",
			ChangedFiles:     changed,
		}
	}

	return Classification{
		Decision:     DecisionSkip,
		Modules:      []string{},
		Reason:       "no deployable path changed",
		ChangedFiles: changed,
	}
}

func matchedModules(files []string, rules Rules) ([]string, []string) {
	serverModules := make([]string, 0)
	nonServerModules := make([]string, 0)

	for _, rule := range rules.Modules {
		if !hasMatch(files, rule.Paths) {
			continue
		}
		if rule.Module == ModuleCloudflare {
			nonServerModules = appendUnique(nonServerModules, rule.Module)
			continue
		}
		serverModules = appendUnique(serverModules, rule.Module)
	}

	return serverModules, nonServerModules
}

func matchedNonServerModules(files []string, rules Rules) []string {
	_, nonServerModules := matchedModules(files, rules)
	return nonServerModules
}

func normalizeFiles(files []string) []string {
	result := make([]string, 0, len(files))
	for _, file := range files {
		normalized := strings.Trim(strings.ReplaceAll(file, "\\", "/"), "/ ")
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

func filterIgnoredFiles(files []string, patterns []string) []string {
	if len(patterns) == 0 {
		return files
	}

	result := make([]string, 0, len(files))
	for _, file := range files {
		if hasMatch([]string{file}, patterns) {
			continue
		}
		result = append(result, file)
	}
	return result
}

func hasMatch(files []string, patterns []string) bool {
	for _, file := range files {
		for _, pattern := range patterns {
			if matchPattern(file, pattern) {
				return true
			}
		}
	}
	return false
}

func matchPattern(file, pattern string) bool {
	pattern = strings.Trim(strings.ReplaceAll(pattern, "\\", "/"), "/ ")
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return file == prefix || strings.HasPrefix(file, prefix+"/")
	}

	matched, err := path.Match(pattern, file)
	return err == nil && matched
}

func appendUnique(values []string, next ...string) []string {
	seen := make(map[string]bool, len(values)+len(next))
	result := make([]string, 0, len(values)+len(next))
	for _, value := range append(values, next...) {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
