package finnhub

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// NewsArticle represents a news article from Finnhub
type NewsArticle struct {
	Category string `json:"category"`
	Datetime int64  `json:"datetime"`
	Headline string `json:"headline"`
	ID       int64  `json:"id"`
	Image    string `json:"image"`
	Related  string `json:"related"`
	Source   string `json:"source"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
}

// NewsSentiment represents news sentiment data
type NewsSentiment struct {
	ArticlesInLastWeek int     `json:"articlesInLastWeek"`
	Buzz               float64 `json:"buzz"`
	WeeklyAverage      float64 `json:"weeklyAverage"`
	CompanyScore       float64 `json:"companyNewsScore"`
	SectorAvgScore     float64 `json:"sectorAverageBullishPercent"`
	SectorAvgNews      float64 `json:"sectorAverageNewsScore"`
	Sentiment          float64 `json:"sentiment"`
	BullishPercent     float64 `json:"bullishPercent"`
	BearishPercent     float64 `json:"bearishPercent"`
}

// SentimentResponse represents the full sentiment response
type SentimentResponse struct {
	Buzz      map[string]float64 `json:"buzz"`
	Sentiment NewsSentiment      `json:"sentiment"`
	Symbol    string             `json:"symbol"`
}

// GetCompanyNews fetches news for a specific company
// from: start date (YYYY-MM-DD)
// to: end date (YYYY-MM-DD)
func (c *Client) GetCompanyNews(symbol string, from, to string) ([]NewsArticle, error) {
	endpoint := fmt.Sprintf("/company-news?symbol=%s&from=%s&to=%s", symbol, from, to)

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get company news for %s: %w", symbol, err)
	}

	var news []NewsArticle
	if err := json.Unmarshal(body, &news); err != nil {
		return nil, fmt.Errorf("failed to parse news response: %w", err)
	}

	return news, nil
}

// GetCompanyNewsRecent fetches news for the last N days
func (c *Client) GetCompanyNewsRecent(symbol string, days int) ([]NewsArticle, error) {
	if days <= 0 {
		days = 7
	}

	to := time.Now().Format("2006-01-02")
	from := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	return c.GetCompanyNews(symbol, from, to)
}

// GetMarketNews fetches general market news
// category: "general", "forex", "crypto", "merger"
func (c *Client) GetMarketNews(category string) ([]NewsArticle, error) {
	if category == "" {
		category = "general"
	}

	endpoint := fmt.Sprintf("/news?category=%s", category)

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get market news: %w", err)
	}

	var news []NewsArticle
	if err := json.Unmarshal(body, &news); err != nil {
		return nil, fmt.Errorf("failed to parse news response: %w", err)
	}

	return news, nil
}

// GetNewsSentiment fetches news sentiment for a company (Premium feature)
// Note: This may not work on free tier
func (c *Client) GetNewsSentiment(symbol string) (*SentimentResponse, error) {
	endpoint := fmt.Sprintf("/news-sentiment?symbol=%s", symbol)

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get news sentiment for %s: %w", symbol, err)
	}

	var sentiment SentimentResponse
	if err := json.Unmarshal(body, &sentiment); err != nil {
		return nil, fmt.Errorf("failed to parse sentiment response: %w", err)
	}

	return &sentiment, nil
}

// GetNewsBatch fetches news for multiple symbols
func (c *Client) GetNewsBatch(symbols []string, limit int) map[string][]NewsArticle {
	if limit <= 0 {
		limit = 5
	}

	result := make(map[string][]NewsArticle)

	for _, symbol := range symbols {
		news, err := c.GetCompanyNewsRecent(symbol, 3) // Last 3 days
		if err != nil {
			log.Printf("⚠️ Failed to get news for %s: %v", symbol, err)
			continue
		}

		// Limit articles per symbol
		if len(news) > limit {
			news = news[:limit]
		}

		result[symbol] = news

		// Rate limit protection
		time.Sleep(100 * time.Millisecond)
	}

	return result
}

// FormatNewsForPrompt formats news articles for AI prompt inclusion
func FormatNewsForPrompt(news []NewsArticle, maxArticles int) string {
	if len(news) == 0 {
		return "No recent news available."
	}

	if maxArticles > 0 && len(news) > maxArticles {
		news = news[:maxArticles]
	}

	var result string
	for i, article := range news {
		timestamp := time.Unix(article.Datetime, 0).Format("2006-01-02 15:04")
		result += fmt.Sprintf("%d. [%s] %s - %s\n", i+1, timestamp, article.Source, article.Headline)
	}

	return result
}
