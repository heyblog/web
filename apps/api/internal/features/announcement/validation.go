package announcement

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"heyblog-api/internal/domain/content"
	"heyblog-api/internal/platform/apperror"
)

func validation(message string) error {
	return apperror.New(apperror.KindValidation, "validation_failed", message)
}
func conflict(code, message string) error { return apperror.New(apperror.KindConflict, code, message) }

func validateInput(input Input) error {
	kind, err := content.ParseKind(input.Kind)
	if err != nil {
		return validation("announcement type is invalid")
	}
	if strings.TrimSpace(input.Title) == "" || utf8.RuneCountInString(input.Title) > 256 {
		return validation("announcement title must contain 1 to 256 characters")
	}
	if input.BodyMarkdown != nil && strings.TrimSpace(*input.BodyMarkdown) == "" {
		return validation("announcement body must not be blank")
	}
	if kind == content.KindBanner && input.Priority != 0 {
		return validation("banner priority must be zero")
	}
	action, err := content.ParseActionType(input.ActionType)
	if err != nil {
		return validation("announcement action is invalid")
	}
	if err := content.ValidateAction(action, input.ActionLabel, input.ActionPath, input.ActionExternalURL); err != nil {
		return validation("announcement action fields are invalid")
	}
	if input.EndsAt != nil && (input.StartsAt == nil || !input.EndsAt.After(*input.StartsAt)) {
		return validation("end time must be after start time")
	}
	return nil
}

func parseVersion(value string) (int64, error) {
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version < 1 || strconv.FormatInt(version, 10) != value {
		return 0, validation("announcement version is invalid")
	}
	return version, nil
}
