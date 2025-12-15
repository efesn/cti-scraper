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
	"github.com/PuerkitoBio/goquery"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Println("Usage:")
		fmt.Println("  go run main.go <url>")
		fmt.Println("  go run main.go -f targets.txt")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -f <file>      Run against multiple URLs from file")
		fmt.Println("  -h, --help     Show this help message")
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

	// save html
	htmlPath := filepath.Join(folder, "output.html")
	os.WriteFile(htmlPath, body, 0644)

	// extract urls
	urlsPath := filepath.Join(folder, "urls.txt")
	err = extractURLs(htmlPath, targetURL, urlsPath)
	if err != nil {
		fmt.Println("URL extraction error:", err)
	} else {
		fmt.Println("Extracted URLs saved to:", urlsPath)
	}

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

func extractURLs(htmlPath string, baseURL string, outputPath string) error {
	file, err := os.Open(htmlPath)
	if err != nil {
		return err
	}
	defer file.Close()

	doc, err := goquery.NewDocumentFromReader(file)
	if err != nil {
		return err
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	seen := make(map[string]bool)
	var results []string

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		href = strings.TrimSpace(href)

		if href == "" {
			return
		}

		parsed, err := url.Parse(href)
		if err != nil {
			return
		}

		absolute := base.ResolveReference(parsed).String()

		if !seen[absolute] {
			seen[absolute] = true
			results = append(results, absolute)
		}
	})

	return os.WriteFile(outputPath, []byte(strings.Join(results, "\n")), 0644)
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
