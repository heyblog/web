package publicview

import "fmt"

type DirectoryStatus string

const (
	DirectoryStatusNormal   DirectoryStatus = "normal"
	DirectoryStatusAbnormal DirectoryStatus = "abnormal"
)

type DirectoryStatusCounts struct {
	Normal   int64 `json:"normal"`
	Abnormal int64 `json:"abnormal"`
}

func directoryStatusFromVisibility(visibility string) (DirectoryStatus, error) {
	switch visibility {
	case "VISIBLE":
		return DirectoryStatusNormal, nil
	case "HIDDEN":
		return DirectoryStatusAbnormal, nil
	default:
		return "", fmt.Errorf("unsupported public directory visibility %q", visibility)
	}
}

func visibilityFromDirectoryStatus(status DirectoryStatus) (string, error) {
	switch status {
	case DirectoryStatusNormal:
		return "VISIBLE", nil
	case DirectoryStatusAbnormal:
		return "HIDDEN", nil
	default:
		return "", fmt.Errorf("unsupported directory status %q", status)
	}
}
