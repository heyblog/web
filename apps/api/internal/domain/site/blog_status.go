package site

type BlogStatus string

const (
	BlogUnreviewed BlogStatus = "UNREVIEWED"
	BlogConfirmed  BlogStatus = "BLOG"
	BlogExcluded   BlogStatus = "NON_BLOG"
)

func (status BlogStatus) IsValid() bool {
	switch status {
	case BlogUnreviewed, BlogConfirmed, BlogExcluded:
		return true
	default:
		return false
	}
}
