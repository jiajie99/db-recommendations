package main

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	browser "github.com/EDDYCJY/fake-useragent"
	"github.com/PuerkitoBio/goquery"
	"github.com/samber/lo"
	"github.com/spf13/viper"
)

func initConfig() {
	viper.SetConfigFile("config.yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln(err)
	}
	ID = viper.GetString("user.id")
	Cookie = viper.GetString("user.cookie")
	MediaType = viper.GetString("result.media_type")
	MinMentionTimes = viper.GetInt("result.mention.min")
	MinScore = viper.GetFloat64("result.score.min")
	SortBy = viper.GetString("result.sort")
	TargetUserID = viper.GetString("target_user.id")
	checkConfig()
}

func checkConfig() {
	if ID == "" || Cookie == "" {
		log.Fatalln("id or cookie is empty")
	}
	if MediaType != "book" && MediaType != "movie" {
		log.Fatalln("media must be movie or book")
	}
}

func main() {
	initConfig()
	var id, cookie string
	flag.StringVar(&id, "id", "", "")
	flag.StringVar(&cookie, "cookie", "", "")
	flag.Parse()
	if id != "" {
		ID = id
	}
	if cookie != "" {
		Cookie = cookie
	}
	printResult()
}

func getMaxStart(total int) int {
	if total <= 30 {
		return 0
	}
	if total%30 != 0 {
		return total - total%30
	}
	return total - 30
}

func getMedias(links []string) []*Media {
	ch := make(chan *Media, len(links))
	wg := &sync.WaitGroup{}
	wg.Add(len(links))
	for i := range links {
		go func(path string) {
			getMediaInfo(path, ch)
		}(links[i])
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var medias []*Media
	for r := range ch {
		if r != nil {
			medias = append(medias, r)
		}
		wg.Done()
	}

	log.Printf("successfully analytics %d %ss you've marked", len(medias), MediaType)
	return medias
}

func getMediasSerial(links []string) []*Media {
	var medias []*Media
	for _, link := range links {
		medias = append(medias, getMediaInfoSerial(link))
		//// 设置随机种子
		//rand.Seed(time.Now().UnixNano())
		//
		//// 生成一个随机的秒数，假设我们希望在 5 到 15 秒之间随机暂停
		//minSleep := 1
		//maxSleep := 3
		//randomSleep := rand.Intn(maxSleep-minSleep+1) + minSleep
		//
		//fmt.Printf("Sleeping for %d seconds...\n", randomSleep)
		//
		//// 暂停程序执行
		//time.Sleep(time.Duration(randomSleep) * time.Second)

	}
	log.Printf("successfully analytics %d %ss you've marked", len(medias), MediaType)
	return medias
}

func buildRelationships(recommendations []*Media) map[string][]string {
	relationships := make(map[string][]string)
	for _, r := range recommendations {
		for _, m := range r.RecommendedMedias {
			name := fmt.Sprintf("《%s》", r.OriginalMedia.Name)
			if v, ok := relationships[m.ID]; ok {
				relationships[m.ID] = append(v, name)
			} else {
				relationships[m.ID] = []string{name}
			}
		}
	}
	return relationships
}

func buildResult(medias []*Media) []MediaInfo {
	var recommendMedias []*MediaInfo
	for _, m := range medias {
		recommendMedias = append(recommendMedias, m.RecommendedMedias...)
	}
	// Remove duplicate medias.
	mediaMap := lo.KeyBy(recommendMedias, func(m *MediaInfo) string {
		return m.ID
	})
	// Get ids.
	ids := lo.Map(recommendMedias, func(m *MediaInfo, _ int) string {
		return m.ID
	})
	relevanceMap := lo.CountValues(ids)
	// Fill relevance.
	for k, v := range relevanceMap {
		mediaMap[k].Relevance = v
	}
	// Fill parents.
	relationShipMap := buildRelationships(medias)
	for k, v := range relationShipMap {
		mediaMap[k].parentMediaNames = v
	}

	mediaSlice := lo.MapToSlice(mediaMap, func(_ string, v *MediaInfo) MediaInfo {
		return *v
	})
	analyticsMediaIDs := lo.Map(medias, func(m *Media, _ int) string {
		return m.OriginalMedia.ID
	})
	// Filter illegal medias.
	mediaSlice = lo.Filter(mediaSlice, func(m MediaInfo, _ int) bool {
		return m.Relevance > MinMentionTimes &&
			m.Rate > MinScore &&
			!slices.Contains(analyticsMediaIDs, m.ID)
	})
	if SortBy == "score" {
		slices.SortFunc(mediaSlice, func(a, b MediaInfo) int {
			if b.Rate == a.Rate {
				return cmp.Compare(b.Relevance, a.Relevance)
			}
			return cmp.Compare(b.Rate, a.Rate)
		})
	} else if SortBy == "mention" {
		slices.SortFunc(mediaSlice, func(a, b MediaInfo) int {
			if b.Relevance == a.Relevance {
				return cmp.Compare(b.Rate, a.Rate)
			}
			return cmp.Compare(b.Relevance, a.Relevance)
		})
	}
	return mediaSlice
}

func printResult() {
	links := prepareMediaLinks()
	medias := getMedias(links)
	mediaSlice := buildResult(medias)
	log.Printf("find %d %ss:\n", len(mediaSlice), MediaType)
	for _, movie := range mediaSlice {
		fmt.Printf("《%s》\n", movie.Name)
		fmt.Printf("Link: %s\n", movie.Link)
		fmt.Printf("Based on: %v\n", movie.parentMediaNames)
		fmt.Printf("Rate: %.1f\n", movie.Rate)
		fmt.Printf("Recommended times: %d\n", movie.Relevance)
		fmt.Printf("\n")
	}
}

func prepareMediaLinks() []string {
	// Get total.
	total := getPersonalMarkMediaTotal()
	maxStart := getMaxStart(total)

	count := maxStart/30 + 1
	ch := make(chan []string, count)
	wg := &sync.WaitGroup{}
	wg.Add(count)
	// Get links.
	for i := 0; i <= maxStart; i += 30 {
		go func(i int) {
			getPersonalMarkMediaLinks(i, ch)
		}(i)
		//// 设置随机种子
		//rand.Seed(time.Now().UnixNano())
		//
		//// 生成一个随机的秒数，假设我们希望在 5 到 15 秒之间随机暂停
		//minSleep := 1
		//maxSleep := 3
		//randomSleep := rand.Intn(maxSleep-minSleep+1) + minSleep
		//
		//fmt.Printf("Sleeping for %d seconds...\n", randomSleep)
		//
		//// 暂停程序执行
		//time.Sleep(time.Duration(randomSleep) * time.Second)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var paths []string
	for result := range ch {
		paths = append(paths, result...)
		wg.Done()
	}

	log.Printf("successfully get %d %ss you've marked", len(paths), MediaType)
	return paths
}

func getRespBody(path string, useCookie, refererURL bool) io.ReadCloser {
	request, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		log.Fatalln(err)
	}
	bid, err := generateRandomBid(11)
	if err != nil {
		log.Fatalln(err)
	}
	request.Header = http.Header{
		"User-Agent": []string{browser.Random()},
		"Accept": []string{
			`text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`,
		},
		//"Accept-Encoding": []string{`gzip, deflate, br, zstd`},
		"Accept-Language": []string{`zh-CN,zh;q=0.9`},
		"Cache-Control":   []string{`max-age=0`},
		"Priority":        []string{`u=0, i`},
		"Cookie":          []string{fmt.Sprintf("bid=%s", bid)},
	}
	if useCookie {
		request.Header["Cookie"] = []string{Cookie}
	}
	if refererURL {
		request.Header["Referer"] = []string{fmt.Sprintf(`https://%s.douban.com/`, MediaType)}
	}
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatalln(err)
	}
	if res.StatusCode != http.StatusOK {
		log.Println("get unexpected status code", res.StatusCode)
	}
	return res.Body
}

func getPersonalMarkMediaTotal() int {
	path := fmt.Sprintf(PersonalMainPageUrl, MediaType, TargetUserID, 0)
	body := getRespBody(path, true, false)
	defer body.Close()

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		log.Fatalln(err)
	}
	//fmt.Println(string(bodyBytes))

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		log.Fatalln(err)
	}

	num := getNum(doc.Find("#db-usr-profile > div.info > h1").Text())
	total, err := strconv.Atoi(num)
	if err != nil {
		log.Fatalln(err)
	}

	return total
}

func getPersonalMarkMediaLinks(start int, ch chan<- []string) {
	path := fmt.Sprintf(PersonalMainPageUrl, MediaType, TargetUserID, start)
	body := getRespBody(path, true, false)
	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		log.Fatalln(err)
	}

	selection := doc.Find("div.item-show > div.title > a")
	result := make([]string, 0, selection.Length())
	selection.Each(func(_ int, s *goquery.Selection) {
		if link, exists := s.Attr("href"); exists {
			result = append(result, link)
		}
	})
	ch <- result
}

func getMediaInfo(path string, ch chan<- *Media) {
	body := getRespBody(path, false, true)
	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		log.Fatalln(err)
	}

	var originalName string
	var sel *goquery.Selection
	switch MediaType {
	case "book":
		originalName = doc.Find("#wrapper > h1 > span").Text()
		sel = doc.Find("#db-rec-section > div > dl")
	case "movie":
		originalName = doc.Find("#content > h1 > span:nth-child(1)").Text()
		sel = doc.Find("#recommendations > div > dl")
	}

	if sel.Length() == 0 {
		log.Printf("get recommended %ss for《%s》failed, link: %s\n", MediaType, originalName, path)
		ch <- nil
		return
	}

	medias := make([]*MediaInfo, 0, sel.Length())
	sel.Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("dd > a").Text())
		if name == "" {
			return
		}
		link, exists := s.Find("dd > a").Attr("href")
		if !exists {
			log.Printf("failed to get link for《%s》\n", name)
			return
		}
		var rate float64
		rateStr := s.Find("dd > span").Text()
		if rateStr != "" {
			rate, _ = strconv.ParseFloat(s.Find("dd > span").Text(), 64)
		}
		medias = append(medias, &MediaInfo{
			ID:   getNum(link),
			Name: name,
			Link: link,
			Rate: rate,
		})
	})

	ch <- &Media{
		OriginalMedia: &MediaInfo{
			ID:   getNum(path),
			Name: originalName,
			Link: path,
		},
		RecommendedMedias: medias,
	}
}

func getMediaInfoSerial(path string) *Media {
	body := getRespBody(path, true, false)
	defer body.Close()

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		log.Fatalln(err)
	}

	var originalName string
	var sel *goquery.Selection
	switch MediaType {
	case "book":
		originalName = doc.Find("#wrapper > h1 > span").Text()
		sel = doc.Find("#db-rec-section > div > dl")
	case "movie":
		originalName = doc.Find("#content > h1 > span:nth-child(1)").Text()
		sel = doc.Find("#recommendations > div > dl")
	}

	if sel.Length() == 0 {
		log.Printf("get recommended %ss for《%s》failed, link: %s\n", MediaType, originalName, path)
		return nil
	}

	medias := make([]*MediaInfo, 0, sel.Length())
	sel.Each(func(i int, s *goquery.Selection) {
		name := strings.TrimSpace(s.Find("dd > a").Text())
		if name == "" {
			return
		}
		link, exists := s.Find("dd > a").Attr("href")
		if !exists {
			log.Printf("failed to get link for《%s》\n", name)
			return
		}
		var rate float64
		rateStr := s.Find("dd > span").Text()
		if rateStr != "" {
			rate, _ = strconv.ParseFloat(s.Find("dd > span").Text(), 64)
		}
		medias = append(medias, &MediaInfo{
			ID:   getNum(link),
			Name: name,
			Link: link,
			Rate: rate,
		})
	})

	return &Media{
		OriginalMedia: &MediaInfo{
			ID:   getNum(path),
			Name: originalName,
			Link: path,
		},
		RecommendedMedias: medias,
	}
}

func getNum(path string) string {
	pattern := `(\d+)`
	re := regexp.MustCompile(pattern)
	return re.FindString(path)
}

func generateRandomBid(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}
