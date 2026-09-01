package content

import "testing"

func TestFiniteAnnouncementValues(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		values  []string
		invalid string
		parse   func(string) error
	}{
		{
			name: "kind", values: []string{"MAIN", "BANNER"}, invalid: "MODAL",
			parse: func(value string) error { _, err := ParseKind(value); return err },
		},
		{
			name: "status", values: []string{"DRAFT", "PUBLISHED", "ARCHIVED"}, invalid: "ACTIVE",
			parse: func(value string) error { _, err := ParseStatus(value); return err },
		},
		{
			name: "action type", values: []string{"NONE", "INTERNAL", "EXTERNAL"}, invalid: "LINK",
			parse: func(value string) error { _, err := ParseActionType(value); return err },
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			for _, value := range testCase.values {
				if err := testCase.parse(value); err != nil {
					t.Errorf("parse %q: %v", value, err)
				}
			}
			if err := testCase.parse(testCase.invalid); err == nil {
				t.Errorf("parse %q error = nil, want invalid value error", testCase.invalid)
			}
		})
	}
}

func TestValidateAction(t *testing.T) {
	t.Parallel()

	label := "Read more"
	blankLabel := " \t "
	path := "/announcements/current?source=home"
	protocolRelativePath := "//example.com/path"
	spacedPath := "/announcement path"
	externalURL := "https://example.com/announcements/current"
	userInfoURL := "https://reader" + "@example.com/announcements/current"
	invalidURL := "mailto:admin@example.com"
	tests := []struct {
		name        string
		actionType  ActionType
		label       *string
		path        *string
		externalURL *string
		wantError   bool
	}{
		{name: "none", actionType: ActionNone},
		{name: "internal", actionType: ActionInternal, label: &label, path: &path},
		{name: "external", actionType: ActionExternal, label: &label, externalURL: &externalURL},
		{name: "unknown type", actionType: "LINK", wantError: true},
		{name: "none with fields", actionType: ActionNone, label: &label, wantError: true},
		{name: "internal missing label", actionType: ActionInternal, path: &path, wantError: true},
		{name: "internal with external URL", actionType: ActionInternal, label: &label, path: &path, externalURL: &externalURL, wantError: true},
		{name: "internal blank label", actionType: ActionInternal, label: &blankLabel, path: &path, wantError: true},
		{name: "protocol-relative internal path", actionType: ActionInternal, label: &label, path: &protocolRelativePath, wantError: true},
		{name: "internal path with spaces", actionType: ActionInternal, label: &label, path: &spacedPath, wantError: true},
		{name: "external missing label", actionType: ActionExternal, externalURL: &externalURL, wantError: true},
		{name: "external with path", actionType: ActionExternal, label: &label, path: &path, externalURL: &externalURL, wantError: true},
		{name: "external user info", actionType: ActionExternal, label: &label, externalURL: &userInfoURL, wantError: true},
		{name: "external non-http URL", actionType: ActionExternal, label: &label, externalURL: &invalidURL, wantError: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateAction(testCase.actionType, testCase.label, testCase.path, testCase.externalURL)
			if testCase.wantError && err == nil {
				t.Fatal("ValidateAction() error = nil, want error")
			}
			if !testCase.wantError && err != nil {
				t.Fatalf("ValidateAction() error = %v", err)
			}
		})
	}
}
