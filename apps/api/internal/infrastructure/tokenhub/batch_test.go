package tokenhub

import "testing"

func TestBatchRejectsMissingUnknownDuplicateAndUnsafeResults(t *testing.T) {
	inputs := []BatchInput{{ID: "a", Name: "生活"}, {ID: "b", Name: "开发"}}
	for _, data := range []string{
		`{"items":[{"id":"a","slug":"life"}]}`,
		`{"items":[{"id":"a","slug":"life"},{"id":"a","slug":"development"}]}`,
		`{"items":[{"id":"a","slug":"life"},{"id":"c","slug":"development"}]}`,
		`{"items":[{"id":"a","slug":"生活"},{"id":"b","slug":"development"}]}`,
		`{"items":[{"id":"a","slug":"life","extra":"ignored"},{"id":"b","slug":"development"}]}`,
		`{"items":[{"id":"a","slug":"life"},{"id":"b","slug":"development"}]} {}`,
	} {
		if _, err := decodeBatch(data, inputs); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	result, err := decodeBatch(`{"items":[{"id":"b","slug":"development"},{"id":"a","slug":"life"}]}`, inputs)
	if err != nil || len(result) != 2 || result[0].ID != "b" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
