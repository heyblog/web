package publicview

import (
	"sync"
	"sync/atomic"
)

type Service struct {
	optionsOnce     sync.Once
	optionsGate     chan struct{}
	optionsSnapshot atomic.Pointer[cachedDirectoryTags]
	graphOnce       sync.Once
	graphGate       chan struct{}
	graphSnapshot   atomic.Pointer[cachedGraph]
	graph           GraphQueries
	home            HomeQueries
	directory       DirectoryQueries
	options         DirectoryOptionsQueries
	random          RandomQueries
	lookup          LookupQueries
	profile         ProfileQueries
	icons           IconQueries
	announcements   AnnouncementReadQueries
	sitemap         SitemapQueries
	metrics         MetricsRecorder
}

func New(queries Queries, metrics MetricsRecorder) *Service {
	return &Service{graph: queries, home: queries, directory: queries, options: queries, random: queries, lookup: queries, profile: queries, icons: queries, announcements: queries, sitemap: queries, metrics: metrics}
}
