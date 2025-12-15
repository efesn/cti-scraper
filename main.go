package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  go run main.go <url>")
		fmt.Println("  go run main.go -f targets.txt")
		return
	}
	// get domains
	var targets []string

	if os.Args[1] == "-f" && len(os.Args) >= 3 {
		file, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Println("File error:", err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				targets = append(targets, line)
			}
		}
	} else {
		targets = append(targets, os.Args[1])
	}

	for _, target := range targets {
		fmt.Println("Processing:", target)
		runTarget(target)
	}
}

func runTarget(targetURL string) {
	parsedURL, err := url.Parse(targetURL)
	baseFolder := "results"

	if err == nil && parsedURL.Host != "" {
		baseFolder = filepath.Join("results", sanitize(parsedURL.Host))
	}

	folder := nextRunFolder(baseFolder)
	os.MkdirAll(folder, 0755)

	// get & download html
	resp, err := http.Get(targetURL)
	if err != nil {
		fmt.Println("Request error:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println("HTTP Error:", resp.Status)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}

	os.WriteFile(filepath.Join(folder, "output.html"), body, 0644)

	// take screenshot
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var screenshot []byte
	err = chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.Sleep(3*time.Second),
		chromedp.FullScreenshot(&screenshot, 90),
	)

	if err != nil {
		fmt.Println("Screenshot error:", err)
		return
	}

	os.WriteFile(filepath.Join(folder, "screenshot.png"), screenshot, 0644)

	fmt.Println("Saved to:", folder)
}

// sanitize replaces characters not suitable for folder names

func sanitize(s string) string {
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}

func nextRunFolder(base string) string {
	os.MkdirAll(base, 0755)

	for i := 1; ; i++ {
		folder := filepath.Join(base, fmt.Sprintf("run-%d", i))
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			return folder
		}
	}
}
