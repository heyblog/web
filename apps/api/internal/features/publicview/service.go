package publicview

type Service struct {
	home      HomeQueries
	directory DirectoryQueries
	options   DirectoryOptionsQueries
	random    RandomQueries
	lookup    LookupQueries
	profile   ProfileQueries
	icons     IconQueries
}

func New(queries Queries) *Service {
	return &Service{home: queries, directory: queries, options: queries, random: queries, lookup: queries, profile: queries, icons: queries}
}
