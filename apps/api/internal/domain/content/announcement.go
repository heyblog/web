package content

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

type Kind string

const (
	KindMain   Kind = "MAIN"
	KindBanner Kind = "BANNER"
)

func ParseKind(value string) (Kind, error) {
	kind := Kind(value)
	if err := kind.Validate(); err != nil {
		return "", err
	}
	return kind, nil
}

func (kind Kind) Validate() error {
	switch kind {
	case KindMain, KindBanner:
		return nil
	default:
		return fmt.Errorf("invalid announcement kind %q", kind)
	}
}

type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusPublished Status = "PUBLISHED"
	StatusArchived  Status = "ARCHIVED"
)

func ParseStatus(value string) (Status, error) {
	status := Status(value)
	if err := status.Validate(); err != nil {
		return "", err
	}
	return status, nil
}

func (status Status) Validate() error {
	switch status {
	case StatusDraft, StatusPublished, StatusArchived:
		return nil
	default:
		return fmt.Errorf("invalid announcement status %q", status)
	}
}

type ActionType string

const (
	ActionNone     ActionType = "NONE"
	ActionInternal ActionType = "INTERNAL"
	ActionExternal ActionType = "EXTERNAL"
)

func ParseActionType(value string) (ActionType, error) {
	actionType := ActionType(value)
	if err := actionType.Validate(); err != nil {
		return "", err
	}
	return actionType, nil
}

func (actionType ActionType) Validate() error {
	switch actionType {
	case ActionNone, ActionInternal, ActionExternal:
		return nil
	default:
		return fmt.Errorf("invalid announcement action type %q", actionType)
	}
}

func ValidateAction(actionType ActionType, label, path, externalURL *string) error {
	if err := actionType.Validate(); err != nil {
		return err
	}

	switch actionType {
	case ActionNone:
		if label != nil || path != nil || externalURL != nil {
			return fmt.Errorf("action type %q does not accept action fields", actionType)
		}
		return nil
	case ActionInternal:
		if label == nil || path == nil || externalURL != nil {
			return fmt.Errorf("action type %q requires label and path only", actionType)
		}
		if err := validateLabel(*label); err != nil {
			return err
		}
		if err := validateInternalPath(*path); err != nil {
			return err
		}
		return nil
	case ActionExternal:
		if label == nil || path != nil || externalURL == nil {
			return fmt.Errorf("action type %q requires label and external URL only", actionType)
		}
		if err := validateLabel(*label); err != nil {
			return err
		}
		if err := validateExternalURL(*externalURL); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid announcement action type %q", actionType)
	}
}

func validateLabel(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("announcement action label must not be blank")
	}
	return nil
}

func validateInternalPath(value string) error {
	if strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsSpace) >= 0 ||
		!strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fmt.Errorf("announcement action path must be a root-relative URL")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return fmt.Errorf("announcement action path must be a root-relative URL")
	}
	return nil
}

func validateExternalURL(value string) error {
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("announcement external action URL must be an absolute HTTP or HTTPS URL")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("announcement external action URL must be an absolute HTTP or HTTPS URL")
	}
	return nil
}
