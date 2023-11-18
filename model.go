package main

type MediaInfo struct {
	ID               string
	Name             string
	Link             string
	Rate             float64
	Relevance        int
	parentMediaNames []string
}

type Media struct {
	OriginalMedia     *MediaInfo
	RecommendedMedias []*MediaInfo
}
