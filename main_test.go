package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetRecommendations(t *testing.T) {
	initConfig()
	ch := make(chan *Media)
	path := "https://book.douban.com/subject/36481438/?icn=index-latestbook-subject"
	go getMediaInfo(path, ch)
	media := <-ch
	close(ch)
	fmt.Printf("%+v\n", media.OriginalMedia)
	for _, m := range media.RecommendedMedias {
		fmt.Printf("%+v\n", m)
	}
}

func TestPrepareMediaLinks(t *testing.T) {
	initConfig()
	prepareMediaLinks()
}

func TestGetAllRecommendations(t *testing.T) {
	initConfig()
	links := []string{
		"https://movie.douban.com/subject/1292063/",
		"https://movie.douban.com/subject/1292052/",
	}
	getMedias(links)
}

func TestGetPersonalMark(t *testing.T) {
	initConfig()
	ch := make(chan []string)
	go getPersonalMarkMediaLinks(1, ch)
	fmt.Println(<-ch)
	close(ch)
}

func TestGetPersonalMarkMediaTotal(t *testing.T) {
	initConfig()
	fmt.Println(getPersonalMarkMediaTotal())
}

func TestGetMaxStart(t *testing.T) {
	tests := []struct {
		input  int
		expect int
	}{
		{
			input:  10,
			expect: 0,
		},
		{
			input:  45,
			expect: 30,
		},
		{
			input:  90,
			expect: 60,
		},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expect, getMaxStart(tt.input))
	}
}
