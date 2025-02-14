package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
	tele "gopkg.in/telebot.v4"
)

// Last time that the rss was fetched
const lastTimestampFile = "last_timestamp.json"

var rssLinks = []string{
	// makeDeviantartRSS("petirep"),
	// makeDeviantartRSS("lemmino"),
	// makeDeviantartRSS("a7md3mad"),
	// makeDeviantartRSS("megatruh"),
	// makeDeviantartRSS("t1na"),
	// makeDeviantartRSS("rhads"),
	// makeDeviantartRSS("pypr"),
	// "https://www.reddit.com/r/ImaginaryLandscapes/.rss",
	"https://www.pinterest.com/tholoooo/art.rss",
	"https://www.pinterest.com/tholoooo/aesthetic.rss",
}

var resolutions = []string{"originals", "1200x", "736x", "564x", "474x", "236x"}

func makeDeviantartRSS(username string) string {
	return "https://backend.deviantart.com/rss.xml?type=deviation&q=by%3A" + username + "+sort%3Atime+meta%3Aall"
}

func getBestImage(thumbURL string) string {
	baseURL := strings.Replace(thumbURL, "236x", "", 1) // Remove the small size reference

	for _, res := range resolutions {
		highResURL := strings.Replace(baseURL, ".com/", fmt.Sprintf(".com/%s/", res), 1)

		// Check if the image exists
		resp, err := http.Head(highResURL)
		if err == nil && resp.StatusCode == 200 {
			return highResURL
		}
	}

	return thumbURL
}

func getNewItems(url string) ([]*gofeed.Item, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
	if err != nil {
		return nil, err
	}

	lastTimestamp := loadLastTimestamp()
	var newItems []*gofeed.Item

	for _, item := range feed.Items {
		if item.PublishedParsed == nil {
			continue
		}
		if item.Image == nil {
			continue
		}

		// If newer than the last processed timestamp, add it
		if item.PublishedParsed.After(lastTimestamp) {
			newItems = append(newItems, item)
		}

		if len(newItems) > 10 || item.PublishedParsed.Year() < time.Now().Year() {
			break
		}
	}

	return newItems, nil
}

func loadLastTimestamp() time.Time {
	data, err := os.ReadFile(lastTimestampFile)
	if err != nil {
		return time.Time{} // Default to zero time (first run)
	}

	var lastTimestamp time.Time
	if err := json.Unmarshal(data, &lastTimestamp); err != nil {
		return time.Time{}
	}

	return lastTimestamp
}

func saveLastTimestamp(timestamp time.Time) {
	data, _ := json.MarshalIndent(timestamp, "", "  ")
	_ = os.WriteFile(lastTimestampFile, data, 0644)
}

func rssPolling(interval time.Duration, c chan *gofeed.Item) {
	log.Println("Started RSS Poller!")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var allNewItems []*gofeed.Item

	for range ticker.C {
		log.Println("Polling...")
		for _, url := range rssLinks {
			newItems, err := getNewItems(url)
			if err != nil {
				log.Println("Couldn't get new items: ", err)
				continue
			}

			log.Printf("Got %d new items from %s", len(newItems), url)
			allNewItems = append(allNewItems, newItems...)
		}
		saveLastTimestamp(time.Now())

		// Shuffle to not send all posts from the same artist at once
		rand.Shuffle(len(allNewItems), func(i, j int) {
			allNewItems[i], allNewItems[j] = allNewItems[j], allNewItems[i]
		})

		for _, item := range allNewItems {
			c <- item
		}
	}

}

func getAuthor(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		log.Fatal(err)
	}

	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}

	userName := parts[0]
	return userName
}

func sendItems(b *tele.Bot, c chan *gofeed.Item) {
	log.Println("Started Sender!")
	chat := &tele.Chat{ID: -1002283087300}
	for item := range c {
		caption := fmt.Sprintf("[%s](%s)", "src", item.Link)

		highQualityURL := getBestImage(item.Image.URL)
		p := &tele.Photo{File: tele.FromURL(highQualityURL), Caption: caption}
		_, _ = b.Send(chat, p, &tele.SendOptions{ParseMode: "markdown"})

		sleepDuration := time.Duration(rand.IntN(30-5) + 5)
		log.Printf("Posted %s - Sleeping for %d seconds.", item.Link, sleepDuration)
		time.Sleep(sleepDuration * time.Second)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Println("Bot Started!")

	c := make(chan *gofeed.Item)
	go rssPolling(10*time.Minute, c)
	go sendItems(b, c)

	b.Start()
}
