package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func main() {
	// Parse flags
	playlistPath := flag.String("file", "playlist.txt", "Youtube playlist link file")
	keyPath := flag.String("keyfile", "key.txt", "API key file")

	// Get API key
	key, err := os.ReadFile(*keyPath)
	if err != nil {
		log.Fatal(err)
	}
	apiKey := string(key)

	// Get playlists
	playlistRaw, err := os.ReadFile(*playlistPath)
	if err != nil {
		log.Fatal(err)
	}
	playlists := strings.Split(string(playlistRaw), "\n")

	// Get playlist IDs
	playlistIds, err := getPlaylistIds(playlists)
	if err != nil {
		log.Fatal(err)
	}

	// Instantiate YT service
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal(err)
	}

	// Merge playlist videos
	list, err := mergePlaylists(service, playlistIds)
	if err != nil {
		log.Fatal(err)
	}

	// Print for CSV
	for _, item := range list {
		fmt.Println("\"", item.Snippet.Title, "\", \"", item.ContentDetails.VideoPublishedAt, "\", \"", item.ContentDetails.VideoId, "\"")
	}

}

type timedItem struct {
	Item youtube.PlaylistItem
	Time time.Time
}

type ByDate []timedItem

func (it ByDate) Len() int      { return len(it) }
func (it ByDate) Swap(i, j int) { it[i], it[j] = it[j], it[i] }
func (it ByDate) Less(i, j int) bool {
	return it[i].Time.Before(it[j].Time)
}

func mergePlaylists(service *youtube.Service, playlistIds []string) ([]youtube.PlaylistItem, error) {
	items, err := getAllItems(service, playlistIds)
	if err != nil {
		return nil, err
	}

	timedItems := make([]timedItem, len(items))
	for i, item := range items {
		time, err := time.Parse(time.RFC3339, item.ContentDetails.VideoPublishedAt)
		if err != nil {
			return nil, err
		}
		timedItems[i] = timedItem{item, time}
	}

	sort.Sort(ByDate(timedItems))

	sortedItems := make([]youtube.PlaylistItem, len(timedItems))
	for i, timedItem := range timedItems {
		sortedItems[i] = timedItem.Item
	}

	return sortedItems, nil
}

func getAllItems(service *youtube.Service, playlistIds []string) ([]youtube.PlaylistItem, error) {
	allItems := []youtube.PlaylistItem{}
	for _, playlistId := range playlistIds {
		plItems, err := getPlaylistItems(service, playlistId)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, plItems...)
	}

	return allItems, nil
}

func getPlaylistItems(service *youtube.Service, playlistId string) ([]youtube.PlaylistItem, error) {
	buf := []youtube.PlaylistItem{}
	nextPageToken := ""

	for {
		res, err := listPlaylist(service, playlistId, nextPageToken)
		if err != nil {
			return nil, err
		}

		for _, item := range res.Items {
			buf = append(buf, *item)
		}

		nextPageToken = res.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	return buf, nil
}

func listPlaylist(service *youtube.Service, playlistId string, pageToken string) (*youtube.PlaylistItemListResponse, error) {
	call := service.PlaylistItems.List([]string{"snippet", "id", "contentDetails"})
	call.PlaylistId(playlistId)
	if pageToken != "" {
		call.PageToken(pageToken)
	}

	res, err := call.Do()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func getPlaylistIds(playlists []string) ([]string, error) {
	// Get playlist IDs
	playlistIds := []string{}
	for _, playlist := range playlists {
		playlistId, err := extractPlaylistId(playlist)
		if err != nil {
			return nil, err
		}
		playlistIds = append(playlistIds, playlistId)
	}

	return playlistIds, nil
}

// Get a playlist ID from a youtube url
func extractPlaylistId(ytUrlString string) (string, error) {
	ytu, err := url.Parse(ytUrlString)
	if err != nil {
		return "", err
	}

	if ytu.Host != "youtube.com" {
		return "", err
	}

	plIds, ok := ytu.Query()["list"]
	if !ok {
		return "", fmt.Errorf("query item 'list' does not exist in url %s", ytUrlString)
	}

	if len(plIds) != 1 {
		return "", errors.New("query item 'list' should have a single value")
	}
	return plIds[0], nil
}
