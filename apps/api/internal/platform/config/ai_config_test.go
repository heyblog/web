package config

import (
	"strings"
	"testing"
)

func TestAIConfigurationUsesYAMLAndOnlyDedicatedEnvironmentSecret(t *testing.T) {
	configuration, err := load(writeConfigFiles(t, testDefaultYAML, nil), func(name string) string {
		if name == "API_TOKENHUB_API_KEY" {
			return "test-tokenhub-key"
		}
		if name == "OPENAI_API_KEY" {
			t.Fatal("read unrelated credential")
		}
		return serviceEnvironment(name)
	})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.AI.APIKey != "test-tokenhub-key" || configuration.AI.DefaultModel != "deepseek/deepseek-flash" || configuration.AI.Limits.Concurrent != 2 {
		t.Fatal("AI config was not loaded")
	}
	without, err := load(writeConfigFiles(t, testDefaultYAML, nil), serviceEnvironment)
	if err != nil || without.AI.APIKey != "" {
		t.Fatalf("missing AI key should allow startup: %v", err)
	}
}

func TestAIConfigurationRejectsUnsafeProviderAndUnboundedPolicy(t *testing.T) {
	for _, override := range []string{
		"base_url: http://provider.test/v1", "base_url: https://user:secret@provider.test/v1", "base_url: https://provider.test/v1?key=secret", "timeout: 16s", "max_output_tokens: 129", "default_model: arbitrary-model", "limits:\n    concurrent: 0", "api_key: forbidden-secret",
		"batch:\n    size: 11\n    max_tags: 500\n    max_output_tokens: 4096", "batch:\n    size: 10\n    max_tags: 501\n    max_output_tokens: 4096", "batch:\n    size: 10\n    max_tags: 500\n    max_output_tokens: 4097",
	} {
		t.Run(strings.Split(override, ":")[0]+override, func(t *testing.T) {
			_, err := load(writeConfigPair(t, testDefaultYAML, "mode: development\nai:\n  "+override+"\n"), serviceEnvironment)
			if err == nil {
				t.Fatal("unsafe config accepted")
			}
		})
	}
}
