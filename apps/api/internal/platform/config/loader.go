package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

func Load() (Config, error) {
	paths, err := locateConfigPaths()
	if err != nil {
		return Config{}, err
	}
	return load(paths, os.Getenv)
}

func locateConfigPaths() (configPaths, error) {
	executable, err := os.Executable()
	if err != nil {
		return configPaths{}, fmt.Errorf("locate API executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return configPaths{}, fmt.Errorf("resolve API executable: %w", err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return configPaths{}, fmt.Errorf("locate API working directory: %w", err)
	}
	return discoverPaths(executable, workingDirectory)
}

func discoverPaths(executable, workingDirectory string) (configPaths, error) {
	executableDirectory := filepath.Join(filepath.Dir(executable), configDirectoryName)
	executableDefault := filepath.Join(executableDirectory, defaultFileName)
	if _, err := os.Stat(executableDefault); err == nil {
		return pathsIn(executableDirectory), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return configPaths{}, fmt.Errorf("inspect executable configuration: %w", err)
	}
	return pathsIn(filepath.Join(workingDirectory, configDirectoryName)), nil
}

func pathsIn(directory string) configPaths {
	return configPaths{
		Default:  filepath.Join(directory, defaultFileName),
		Override: filepath.Join(directory, overrideFileName),
	}
}

func load(paths configPaths, getenv getenvFunc) (Config, error) {
	base, err := readYAML(paths.Default)
	if err != nil {
		return Config{}, fmt.Errorf("read default configuration: %w", err)
	}
	if mappingValueIndex(base, "mode") >= 0 {
		return Config{}, fmt.Errorf("default configuration must not define mode")
	}
	override, err := readYAML(paths.Override)
	if errors.Is(err, os.ErrNotExist) {
		base.Content = append(base.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mode"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: string(ModeDevelopment)},
		)
	} else if err != nil {
		return Config{}, fmt.Errorf("read configuration override: %w", err)
	} else {
		if mappingValueIndex(override, "mode") < 0 {
			return Config{}, fmt.Errorf("configuration override must define mode")
		}
		mergeYAML(base, override)
	}

	fileValues, err := decodeFileConfig(base)
	if err != nil {
		return Config{}, err
	}
	return resolve(fileValues, getenv)
}

func readYAML(path string) (*yaml.Node, error) {
	contents, err := os.ReadFile(path) // #nosec G304 -- configuration paths are derived from the executable or working directory.
	if err != nil {
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode %s: multiple YAML documents are not allowed", path)
		}
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("decode %s: root must be a mapping", path)
	}
	if err := validateYAMLNode(document.Content[0], ""); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return document.Content[0], nil
}

func validateYAMLNode(node *yaml.Node, path string) error {
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("%s: aliases are not allowed", displayPath(path))
	}
	if node.Tag == "!!null" {
		return fmt.Errorf("%s: null values are not allowed", displayPath(path))
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode || key.Value == "" {
				return fmt.Errorf("%s: mapping keys must be non-empty strings", displayPath(path))
			}
			if _, exists := seen[key.Value]; exists {
				return fmt.Errorf("%s: duplicate field", joinPath(path, key.Value))
			}
			seen[key.Value] = struct{}{}
			if err := validateYAMLNode(node.Content[index+1], joinPath(path, key.Value)); err != nil {
				return err
			}
		}
		return nil
	}
	for index, child := range node.Content {
		if err := validateYAMLNode(child, joinPath(path, strconv.Itoa(index))); err != nil {
			return err
		}
	}
	return nil
}

func mergeYAML(base, override *yaml.Node) {
	for index := 0; index < len(override.Content); index += 2 {
		key := override.Content[index]
		value := override.Content[index+1]
		baseIndex := mappingValueIndex(base, key.Value)
		if baseIndex < 0 {
			base.Content = append(base.Content, cloneYAMLNode(key), cloneYAMLNode(value))
			continue
		}
		baseValue := base.Content[baseIndex]
		if baseValue.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode {
			mergeYAML(baseValue, value)
			continue
		}
		base.Content[baseIndex] = cloneYAMLNode(value)
	}
}

func mappingValueIndex(mapping *yaml.Node, key string) int {
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return index + 1
		}
	}
	return -1
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	clone := *node
	clone.Content = make([]*yaml.Node, len(node.Content))
	for index, child := range node.Content {
		clone.Content[index] = cloneYAMLNode(child)
	}
	return &clone
}

func decodeFileConfig(root *yaml.Node) (fileConfig, error) {
	document := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	contents, err := yaml.Marshal(document)
	if err != nil {
		return fileConfig{}, fmt.Errorf("encode merged configuration: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	var values fileConfig
	if err := decoder.Decode(&values); err != nil {
		return fileConfig{}, fmt.Errorf("decode merged configuration: %w", err)
	}
	if values.Version != configVersion {
		return fileConfig{}, fmt.Errorf("configuration version must be %d", configVersion)
	}
	return values, nil
}
