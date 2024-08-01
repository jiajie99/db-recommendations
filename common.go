package main

var (
	ID, Cookie, MediaType, SortBy string
	MinMentionTimes               int
	MinScore                      float64
)

const (
	UserAgent           = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
	PersonalMainPageUrl = "https://%s.douban.com/people/%s/collect?sort=time&amp;start=%d&amp;filter=all&amp;mode=list&amp;tags_sort=count"
)
