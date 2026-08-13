package ai

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ImageGenerator handles generating cover images using AI or stock fallbacks.
type ImageGenerator interface {
	// Generate returns a cover image URL based on the title and category.
	Generate(ctx context.Context, title, category string) string
}

type openAIImageGenerator struct {
	apiKey        string
	imageEndpoint string
	enabled       bool
	// unsupported is set once the upstream reports it cannot generate images
	// (404/405/501), so later calls skip straight to the stock fallback.
	unsupported bool
	client      *http.Client
	logger      *log.Logger
}

// NewImageGenerator creates an ImageGenerator client. When enabled is false,
// generation is skipped entirely and only the stock fallback is used.
func NewImageGenerator(apiKey, baseURL string, enabled bool, logger *log.Logger) ImageGenerator {
	if logger == nil {
		logger = log.Default()
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/images/generations"
	return &openAIImageGenerator{
		apiKey:        apiKey,
		imageEndpoint: endpoint,
		enabled:       enabled,
		client:        &http.Client{Timeout: 30 * time.Second},
		logger:        logger,
	}
}

type imageReq struct {
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
	Model  string `json:"model"`
}

type imageResp struct {
	Data []struct {
		URL string `json:"url"`
	} `json:"data"`
}

func (g *openAIImageGenerator) Generate(ctx context.Context, title, category string) string {
	// Skip AI generation when disabled by config or after the provider
	// reported it does not support image generation.
	if !g.enabled || g.unsupported {
		return GetDynamicUnsplashURL(category, title)
	}

	apiKey, baseURL, _ := resolveAIConfig(g.apiKey, "", g.imageEndpoint)

	imageEndpoint := g.imageEndpoint
	if baseURL != "" {
		imageEndpoint = strings.TrimRight(baseURL, "/") + "/images/generations"
	}

	// 1. Try DALL-E generation if API key is configured
	if apiKey != "" {
		prompt := fmt.Sprintf(
			"A high-quality, professional, modern minimalist technology featured cover image illustrating: %s. Dark cybernetic tech aesthetic.",
			title,
		)
		url, unsupported, ok := generateImage(ctx, g.client, g.logger, imageEndpoint, apiKey, prompt)
		if unsupported {
			g.unsupported = true
			g.logger.Printf("ai: image generation not supported by provider; skipping future attempts")
		}
		if ok {
			return url
		}
	}

	// 2. Fallback to dynamic, unique Unsplash featured images based on title keywords and hash signature.
	// This avoids backend rate-limiting, network latency, and ensures highly relevant, unique cover images.
	return GetDynamicUnsplashURL(category, title)
}

// extractKeywords extracts clean, meaningful keywords from the article title.
func extractKeywords(title string) []string {
	words := strings.Fields(strings.ToLower(title))
	var filtered []string
	stopwords := map[string]bool{
		"how": true, "what": true, "why": true, "with": true, "from": true,
		"this": true, "that": true, "your": true, "over": true, "under": true,
		"their": true, "about": true, "using": true, "built": true, "build": true,
		"and": true, "the": true, "for": true, "our": true, "new": true, "more": true,
	}
	for _, w := range words {
		w = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, w)
		if len(w) > 2 && !stopwords[w] {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

// GetDynamicUnsplashURL returns a client-side resolvable Unsplash featured URL with unique signature and title keywords.
func GetDynamicUnsplashURL(category, title string) string {
	categoryClean := strings.ToLower(strings.TrimSpace(category))
	if categoryClean == "" {
		categoryClean = "technology"
	}

	keywords := extractKeywords(title)
	queryTerms := []string{categoryClean}

	// Add up to 2 title keywords for relevance
	count := 0
	for _, k := range keywords {
		if k != categoryClean {
			queryTerms = append(queryTerms, k)
			count++
			if count >= 2 {
				break
			}
		}
	}

	// Ensure technology context
	queryTerms = append(queryTerms, "tech")

	queryString := strings.Join(queryTerms, ",")
	sig := hashString(title)

	return fmt.Sprintf("https://images.unsplash.com/featured/800x450/?%s&sig=%d", queryString, sig)
}

// hashString computes a simple hash of a string for distribution.
func hashString(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = 31*h + int(s[i])
	}
	if h < 0 {
		h = -h
	}
	return h
}

// GetCuratedTechImage returns a high-resolution, curated Unsplash technology photo URL matching the category, distributed stably by title hash.
func GetCuratedTechImage(category, title string) string {
	categoryClean := strings.ToLower(strings.TrimSpace(category))
	list, ok := stockImages[categoryClean]
	if !ok || len(list) == 0 {
		list = stockImages["default"]
	}

	idx := hashString(title) % len(list)
	return list[idx]
}

var stockImages = map[string][]string{
	"ai": {
		"https://images.unsplash.com/photo-1677442136019-21780efad99a?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1620712943543-bcc4688e7485?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1507146426996-ef05306b995a?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1526374965328-7f61d4dc18c5?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1680814909117-64ecfdf49a5b?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1531746790731-6c087fecd39a?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1558494949-ef010cbdcc31?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1581091226825-a6a2a5aee158?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1579567761406-468a7888ae5f?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1606159068539-43f36b99d1b2?w=800&auto=format&fit=crop&q=80",
	},
	"tools": {
		"https://images.unsplash.com/photo-1515879218367-8466d910aaa4?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1607799279861-4dd421887fb3?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1555066931-4365d14bab8c?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1498050108023-c5249f4df085?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1542831371-29b0f74f9713?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1517694712202-14dd9538aa97?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1605379399642-870262d3d051?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1587831990711-23ca6441447b?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1629654297299-c8506221ca97?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1618477388954-7852f32655ec?w=800&auto=format&fit=crop&q=80",
	},
	"research": {
		"https://images.unsplash.com/photo-1451187580459-43490279c0fa?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1532094349884-543bc11b234d?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1507668077129-56e32842fceb?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1488590528505-98d2b5aba04b?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1461749280684-dccba630e2f6?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1526379095098-d400fd0bf935?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1504384308090-c894fdcc538d?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1518770660439-4636190af475?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1485827404703-89b55fcc595e?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1531297484001-80022131f5a1?w=800&auto=format&fit=crop&q=80",
	},
	"machine-learning": {
		"https://images.unsplash.com/photo-1527474305487-b87b222841cc?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1555255707-c07966088b7b?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1509228468518-180dd4864904?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1580894894513-541e068a3e2b?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1516110833967-0b5716ca1387?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1551288049-bebda4e38f71?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1460925895917-afdab827c52f?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1504868584819-f8e8b4b6d7e3?w=800&auto=format&fit=crop&q=80",
	},
	"industry": {
		"https://images.unsplash.com/photo-1486406146926-c627a92ad1ab?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1444653614773-995cb1ef9efa?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1556761175-5973dc0f32e7?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1553877522-43269d4ea984?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1560179707-f14e90ef3623?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1573164713988-8665fc963095?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1454165804606-c3d57bc86b40?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=800&auto=format&fit=crop&q=80",
	},
	"default": {
		"https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1518770660439-4636190af475?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1485827404703-89b55fcc595e?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1531297484001-80022131f5a1?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1550751827-4bd374c3f58b?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1487058792275-0ad4cadbe4b0?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1519389950473-47ba0277781c?w=800&auto=format&fit=crop&q=80",
		"https://images.unsplash.com/photo-1600132806370-bf17e65e942f?w=800&auto=format&fit=crop&q=80",
	},
}
