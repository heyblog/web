package dataimport

type GraphBundle struct {
	Format      string             `json:"format"`
	Version     int                `json:"version"`
	GeneratedAt string             `json:"generated_at"`
	Inputs      []InputMetadata    `json:"inputs"`
	NodeCount   int                `json:"node_count"`
	EdgeCount   int                `json:"edge_count"`
	Count       int                `json:"count"`
	Links       []FriendLinkSource `json:"links"`
}

type FriendLinkSource struct {
	Source       string   `json:"source"`
	Destinations []string `json:"destinations"`
}
