package publicview

type Service struct {
	graph         GraphQueries
	home          HomeQueries
	directory     DirectoryQueries
	options       DirectoryOptionsQueries
	random        RandomQueries
	lookup        LookupQueries
	profile       ProfileQueries
	icons         IconQueries
	announcements AnnouncementReadQueries
	sitemap       SitemapQueries
}

func New(queries Queries) *Service {
	return &Service{graph: queries, home: queries, directory: queries, options: queries, random: queries, lookup: queries, profile: queries, icons: queries, announcements: queries, sitemap: queries}
}
