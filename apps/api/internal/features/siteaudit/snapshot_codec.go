package siteaudit

import (
	"encoding/json"
	"fmt"
)

func decodeSnapshot(data []byte, destination *Snapshot) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode snapshot JSON: %w", err)
	}
	return nil
}

func encodeSnapshot(snapshot Snapshot) ([]byte, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot JSON: %w", err)
	}
	return data, nil
}

func optionalSnapshotJSON(action Action, snapshot Snapshot) ([]byte, error) {
	if action == ActionCreate {
		return nil, nil
	}
	return encodeSnapshot(snapshot)
}

func baseRevisionPointer(action Action, snapshot Snapshot) *int64 {
	if action == ActionCreate {
		return nil
	}
	value := snapshot.Revision
	return &value
}

// Historical selected-name identifiers support identity resolution when reading
// persisted tag snapshots. TagSnapshot defines the current serialized fields.
func (tag *TagSnapshot) UnmarshalJSON(data []byte) error {
	type current TagSnapshot
	var stored struct {
		current
		LabelID string `json:"label_id"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	*tag = TagSnapshot(stored.current)
	tag.historicalLabelID = stored.LabelID
	return nil
}
