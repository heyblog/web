package site

type BlogStatus string

const (
	BlogUnreviewed BlogStatus = "UNREVIEWED"
	BlogConfirmed  BlogStatus = "BLOG"
	BlogHybrid     BlogStatus = "HYBRID"
	BlogExcluded   BlogStatus = "NON_BLOG"
)

func (status BlogStatus) IsValid() bool {
	switch status {
	case BlogUnreviewed, BlogConfirmed, BlogHybrid, BlogExcluded:
		return true
	default:
		return false
	}
}
