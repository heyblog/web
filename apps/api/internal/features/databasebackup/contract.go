package databasebackup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/google/uuid"
)

type document struct {
	header   Header
	manifest []Manifest
	offsets  map[string]int64
}

type rowVisitor func(string, json.RawMessage) error

func readDocument(reader io.Reader, visit rowVisitor) (document, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	doc := document{offsets: make(map[string]int64)}
	actual := make(map[string]Manifest)
	sections := make(map[string]bool)
	err := object(decoder, func(section string) error {
		sections[section] = true
		switch section {
		case "metadata":
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return ErrInvalid
			}
			return strictDecode(raw, &doc.header)
		case "manifest":
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return ErrInvalid
			}
			return strictDecode(raw, &doc.manifest)
		case "tables", "graph":
			return object(decoder, func(name string) error {
				if section == "graph" {
					if name != "vertices" && name != "edges" {
						return ErrInvalid
					}
					name = "graph." + name
				} else if !slices.Contains(tableNames, name) {
					return ErrInvalid
				}
				if err := delimiter(decoder, '['); err != nil {
					return err
				}
				doc.offsets[name] = decoder.InputOffset() - 1
				entry, err := readRowsContents(decoder, name, visit)
				actual[name] = entry
				return err
			})
		default:
			return ErrInvalid
		}
	})
	if err != nil {
		return document{}, err
	}
	if err := requireEOF(decoder); err != nil {
		return document{}, err
	}
	if len(sections) != 4 || doc.header.Format != Format || doc.header.Version != 1 || doc.header.SchemaVersion != 2 || doc.header.GeneratedAt.IsZero() {
		return document{}, ErrInvalid
	}
	if _, err := uuid.Parse(doc.header.ExcludedSystemAdminID); err != nil {
		return document{}, ErrInvalid
	}
	if len(actual) != len(tableNames)+2 || len(doc.manifest) != len(actual) {
		return document{}, ErrInvalid
	}
	seen := make(map[string]bool)
	for _, entry := range doc.manifest {
		if seen[entry.Name] || actual[entry.Name] != entry {
			return document{}, ErrInvalid
		}
		seen[entry.Name] = true
	}
	return doc, nil
}

func readRows(decoder *json.Decoder, name string, visit rowVisitor) (Manifest, error) {
	if err := delimiter(decoder, '['); err != nil {
		return Manifest{}, err
	}
	return readRowsContents(decoder, name, visit)
}

func readRowsContents(decoder *json.Decoder, name string, visit rowVisitor) (Manifest, error) {
	entry := Manifest{Dataset: Dataset{Name: name}}
	hash := sha256.New()
	for decoder.More() {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return entry, ErrInvalid
		}
		canonical, err := canonicalRow(raw)
		if err != nil {
			return entry, ErrInvalid
		}
		if _, err := hash.Write(append(canonical, '\n')); err != nil {
			return entry, fmt.Errorf("hash row: %w", err)
		}
		if visit != nil {
			if err := visit(name, canonical); err != nil {
				return entry, err
			}
		}
		entry.Count++
	}
	entry.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return entry, delimiter(decoder, ']')
}

func (doc document) inspection(checksum, actor string) Inspection {
	preview := Inspection{
		SHA256: checksum, Version: doc.header.Version, SchemaVersion: doc.header.SchemaVersion,
		GeneratedAt: doc.header.GeneratedAt, ExcludedSystemAdminID: doc.header.ExcludedSystemAdminID,
		RetainedSystemAdminID: actor, Datasets: make([]Dataset, 0, len(tableNames)), Issues: []Issue{},
	}
	for _, entry := range doc.manifest {
		switch entry.Name {
		case "graph.vertices":
			preview.Graph.Vertices = entry.Count
		case "graph.edges":
			preview.Graph.Edges = entry.Count
		default:
			preview.Datasets = append(preview.Datasets, entry.Dataset)
		}
	}
	return preview
}
