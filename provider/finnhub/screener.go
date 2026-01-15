package finnhub

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// Quote represents a stock quote from Finnhub
type Quote struct {
	Symbol        string  `json:"symbol,omitempty"`
	CurrentPrice  float64 `json:"c"`  // Current price
	Change        float64 `json:"d"`  // Change
	PercentChange float64 `json:"dp"` // Percent change
	High          float64 `json:"h"`  // High price of the day
	Low           float64 `json:"l"`  // Low price of the day
	Open          float64 `json:"o"`  // Open price of the day
	PreviousClose float64 `json:"pc"` // Previous close price
	Timestamp     int64   `json:"t"`  // Timestamp
}

// StockSymbol represents a stock symbol from Finnhub
type StockSymbol struct {
	Currency    string `json:"currency"`
	Description string `json:"description"`
	DisplayName string `json:"displaySymbol"`
	FIGI        string `json:"figi"`
	MIC         string `json:"mic"`
	Symbol      string `json:"symbol"`
	Type        string `json:"type"`
}

// ScreenedStock represents a stock with screening data
type ScreenedStock struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	PercentChange float64 `json:"percent_change"`
	Volume        float64 `json:"volume,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

// MarketStatus represents market open/close status
type MarketStatus struct {
	Exchange string `json:"exchange"`
	Holiday  string `json:"holiday"`
	IsOpen   bool   `json:"isOpen"`
	Session  string `json:"session"`
	Timezone string `json:"timezone"`
	T        int64  `json:"t"`
}

// GetQuote fetches real-time quote for a symbol
func (c *Client) GetQuote(symbol string) (*Quote, error) {
	endpoint := fmt.Sprintf("/quote?symbol=%s", strings.ToUpper(symbol))

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote for %s: %w", symbol, err)
	}

	var quote Quote
	if err := json.Unmarshal(body, &quote); err != nil {
		return nil, fmt.Errorf("failed to parse quote response: %w", err)
	}

	quote.Symbol = symbol
	return &quote, nil
}

// GetQuotesBatch fetches quotes for multiple symbols (max 10 per call recommended)
func (c *Client) GetQuotesBatch(symbols []string) (map[string]*Quote, error) {
	results := make(map[string]*Quote)

	for _, symbol := range symbols {
		quote, err := c.GetQuote(symbol)
		if err != nil {
			log.Printf("⚠️ Failed to get quote for %s: %v", symbol, err)
			continue
		}
		results[symbol] = quote
	}

	return results, nil
}

// GetUSStockSymbols fetches all US stock symbols
func (c *Client) GetUSStockSymbols() ([]StockSymbol, error) {
	endpoint := "/stock/symbol?exchange=US"

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get US stock symbols: %w", err)
	}

	var symbols []StockSymbol
	if err := json.Unmarshal(body, &symbols); err != nil {
		return nil, fmt.Errorf("failed to parse symbols response: %w", err)
	}

	return symbols, nil
}

// GetMarketStatus checks if the US market is open
func (c *Client) GetMarketStatus() (*MarketStatus, error) {
	endpoint := "/stock/market-status?exchange=US"

	body, err := c.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get market status: %w", err)
	}

	var status MarketStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse market status: %w", err)
	}

	return &status, nil
}

// GetTopGainers returns the top gaining stocks from a watchlist
// Since Finnhub free tier doesn't have a built-in screener,
// we use a curated list of popular stocks and sort by performance
func (c *Client) GetTopGainers(limit int) ([]ScreenedStock, error) {
	return c.screenStocks("gainers", limit)
}

// GetTopLosers returns the top losing stocks
func (c *Client) GetTopLosers(limit int) ([]ScreenedStock, error) {
	return c.screenStocks("losers", limit)
}

// GetTopMomentum returns stocks with the highest absolute movement
func (c *Client) GetTopMomentum(limit int) ([]ScreenedStock, error) {
	return c.screenStocks("momentum", limit)
}

// Popular US stocks for screening (free tier workaround)
var popularStocks = []string{
	// Tech Giants
	"AAPL", "MSFT", "GOOGL", "AMZN", "META", "NVDA", "TSLA",
	// Semiconductors
	"AMD", "INTC", "AVGO", "QCOM", "MU", "TSM", "ARM",
	// Software
	"CRM", "ORCL", "ADBE", "NOW", "SNOW", "PLTR", "DDOG",
	// Fintech/Crypto
	"COIN", "SQ", "HOOD", "PYPL", "V", "MA",
	// E-commerce/Consumer
	"SHOP", "NFLX", "DIS", "NKE", "SBUX", "MCD", "KO",
	// EV/Clean Energy
	"RIVN", "LCID", "F", "GM", "ENPH", "FSLR",
	// Biotech/Health
	"MRNA", "PFE", "JNJ", "UNH", "LLY", "ABBV",
	// Finance
	"JPM", "BAC", "GS", "MS", "BLK", "C",
	// Energy
	"XOM", "CVX", "SLB", "OXY",
	// Index ETFs (for reference)
	"SPY", "QQQ", "IWM",
}

func (c *Client) screenStocks(screenType string, limit int) ([]ScreenedStock, error) {
	if limit <= 0 {
		limit = 10
	}

	log.Printf("🔍 Screening stocks (type: %s, limit: %d)...", screenType, limit)

	// Fetch quotes for all popular stocks
	var stocks []ScreenedStock

	for _, symbol := range popularStocks {
		quote, err := c.GetQuote(symbol)
		if err != nil {
			continue
		}

		// Skip stocks with no price data
		if quote.CurrentPrice == 0 {
			continue
		}

		stocks = append(stocks, ScreenedStock{
			Symbol:        symbol,
			Name:          symbol, // Finnhub doesn't return name in quote
			Price:         quote.CurrentPrice,
			Change:        quote.Change,
			PercentChange: quote.PercentChange,
		})

		// Small delay to respect rate limits (60/min = 1 per second)
		time.Sleep(100 * time.Millisecond)
	}

	// Sort based on screen type
	switch screenType {
	case "gainers":
		sort.Slice(stocks, func(i, j int) bool {
			return stocks[i].PercentChange > stocks[j].PercentChange
		})
	case "losers":
		sort.Slice(stocks, func(i, j int) bool {
			return stocks[i].PercentChange < stocks[j].PercentChange
		})
	case "momentum":
		sort.Slice(stocks, func(i, j int) bool {
			return abs(stocks[i].PercentChange) > abs(stocks[j].PercentChange)
		})
	}

	// Limit results
	if len(stocks) > limit {
		stocks = stocks[:limit]
	}

	log.Printf("✓ Screened %d stocks", len(stocks))
	return stocks, nil
}

// GetTopRatedStocks returns top stocks for AI candidate selection
// This is the main method used by the kernel engine, similar to GetTopRatedCoins
func (c *Client) GetTopRatedStocks(limit int) ([]string, error) {
	stocks, err := c.GetTopGainers(limit)
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, stock := range stocks {
		symbols = append(symbols, stock.Symbol)
	}

	return symbols, nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
