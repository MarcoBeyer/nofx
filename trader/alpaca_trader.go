package trader

import (
	"fmt"
	"math"
	"nofx/logger"
	"sync"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/shopspring/decimal"
)

// AlpacaTrader implements Trader interface for Alpaca (Stocks)
type AlpacaTrader struct {
	client    *alpaca.Client
	mdClient  *marketdata.Client
	apiKey    string
	secretKey string
	feedURL   string

	// Cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// NewAlpacaTrader creates a new Alpaca trader
func NewAlpacaTrader(apiKey, secretKey, feedURL string) *AlpacaTrader {
	// Initialize Alpaca client
	// feedURL determines Paper or Live
	// Default to Paper if not specified or specific Paper URL
	baseURL := "https://paper-api.alpaca.markets"
	if feedURL != "" {
		baseURL = feedURL
	}

	client := alpaca.NewClient(alpaca.ClientOpts{
		APIKey:    apiKey,
		APISecret: secretKey,
		BaseURL:   baseURL,
	})

	mdClient := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    apiKey,
		APISecret: secretKey,
	})

	trader := &AlpacaTrader{
		client:        client,
		mdClient:      mdClient,
		apiKey:        apiKey,
		secretKey:     secretKey,
		feedURL:       baseURL,
		cacheDuration: 5 * time.Second, // Shorter cache for stocks
	}

	logger.Infof("🔵 [Alpaca] Trader initialized (BaseURL: %s)", baseURL)
	return trader
}

// GetBalance retrieves account balance
func (t *AlpacaTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		balance := t.cachedBalance
		t.balanceCacheMutex.RUnlock()
		return balance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Call API
	acct, err := t.client.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to get Alpaca account: %w", err)
	}

	// Calculate values
	equity, _ := acct.Equity.Float64()
	cash, _ := acct.Cash.Float64()
	buyingPower, _ := acct.BuyingPower.Float64()

	// Alpaca PnL logic
	// Unrealized PnL = Equity - LastEquity (approximate for day) OR iterate positions
	// Better to sum up positions for precise unrealized PnL

	balance := map[string]interface{}{
		"totalEquity":           equity,
		"availableBalance":      buyingPower, // Buying power for margin
		"balance":               cash,
		"totalUnrealizedProfit": 0.0, // Will be updated by GetPositions mostly
	}

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return balance, nil
}

// GetPositions retrieves all open positions
func (t *AlpacaTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := t.client.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get Alpaca positions: %w", err)
	}

	var parsedPositions []map[string]interface{}
	var totalUnrealizedPnL float64

	for _, pos := range positions {
		qty, _ := pos.Qty.Float64()
		entryPrice, _ := pos.AvgEntryPrice.Float64()
		currentPrice, _ := pos.CurrentPrice.Float64()
		unrealizedPnL, _ := pos.UnrealizedPL.Float64()
		unrealizedPnLPct, _ := pos.UnrealizedPLPC.Float64()

		// Convert Alpaca pct (0.05) to percentage (5.0) for consistency
		unrealizedPnLPct = unrealizedPnLPct * 100

		side := "long"
		if pos.Side == "short" {
			side = "short"
			qty = math.Abs(qty)
		}

		position := map[string]interface{}{
			"symbol":           pos.Symbol,
			"side":             side,
			"positionAmt":      qty,
			"entryPrice":       entryPrice,
			"markPrice":        currentPrice, // Using current price as mark price
			"unRealizedProfit": unrealizedPnL,
			"unrealizedPnL":    unrealizedPnL,
			"unrealizedPnLPct": unrealizedPnLPct,
			"leverage":         1.0, // Default to 1 (Stock trading usually) or calculate based on margin
			// Alpaca doesn't explicitly give liquidation price in this struct usually
			"liquidationPrice": 0.0,
		}

		parsedPositions = append(parsedPositions, position)
		totalUnrealizedPnL += unrealizedPnL
	}

	// Piggyback update balance cache if possible
	t.balanceCacheMutex.Lock()
	if t.cachedBalance != nil {
		t.cachedBalance["totalUnrealizedProfit"] = totalUnrealizedPnL
	}
	t.balanceCacheMutex.Unlock()

	return parsedPositions, nil
}

// OpenLong opens a long position (Buy)
func (t *AlpacaTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Format quantity: Alpaca allows fractional shares for many assets, but let's stick to safe formatting
	qtyDec := decimal.NewFromFloat(quantity)

	req := alpaca.PlaceOrderRequest{
		Symbol:      symbol,
		Qty:         &qtyDec,
		Side:        alpaca.Buy,
		Type:        alpaca.Market, // Using Market orders for simplicity
		TimeInForce: alpaca.GTC,
	}

	logger.Infof("[Alpaca] Opening Long %s Qty: %.4f", symbol, quantity)

	order, err := t.client.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("Alpaca OpenLong failed: %w", err)
	}

	return map[string]interface{}{
		"orderId": order.ID,
		"status":  "NEW",
	}, nil
}

// OpenShort opens a short position (Sell)
func (t *AlpacaTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	qtyDec := decimal.NewFromFloat(quantity)

	req := alpaca.PlaceOrderRequest{
		Symbol:      symbol,
		Qty:         &qtyDec,
		Side:        alpaca.Sell,
		Type:        alpaca.Market,
		TimeInForce: alpaca.GTC,
	}

	logger.Infof("[Alpaca] Opening Short %s Qty: %.4f", symbol, quantity)

	order, err := t.client.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("Alpaca OpenShort failed: %w", err)
	}

	return map[string]interface{}{
		"orderId": order.ID,
		"status":  "NEW",
	}, nil
}

// CloseLong closes a long position (Sell)
func (t *AlpacaTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0 or very close to full position, allow ClosePosition API
	// If quantity is 0 or very close to full position, close full position
	if quantity <= 0 {
		logger.Infof("[Alpaca] Closing all Long position for %s", symbol)
		pos, err := t.client.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position for close: %w", err)
		}

		qtyStr := pos.Qty.String()
		qtyDec, _ := decimal.NewFromString(qtyStr)

		// If short, we need to buy to close, but CloseLong assumes we have a long position?
		// Verification: GetPosition returns the net position. We should check if it's actually Long.
		if pos.Side != "long" {
			// Not a long position, nothing to close or it's short
			return map[string]interface{}{"status": "CLOSED", "info": "No long position found"}, nil
		}

		req := alpaca.PlaceOrderRequest{
			Symbol:      symbol,
			Qty:         &qtyDec,
			Side:        alpaca.Sell,
			Type:        alpaca.Market, // Using Market orders for simplicity
			TimeInForce: alpaca.GTC,
		}

		order, err := t.client.PlaceOrder(req)
		if err != nil {
			return nil, fmt.Errorf("Alpaca CloseAllLong failed: %w", err)
		}
		return map[string]interface{}{"orderId": order.ID, "status": "NEW"}, nil
	}

	// Partial close
	qtyDec := decimal.NewFromFloat(quantity)
	req := alpaca.PlaceOrderRequest{
		Symbol:      symbol,
		Qty:         &qtyDec,
		Side:        alpaca.Sell, // Sell to close Long
		Type:        alpaca.Market,
		TimeInForce: alpaca.GTC,
	}

	logger.Infof("[Alpaca] Closing Long %s Qty: %.4f", symbol, quantity)
	order, err := t.client.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("Alpaca CloseLong failed: %w", err)
	}

	return map[string]interface{}{
		"orderId": order.ID,
		"status":  "NEW",
	}, nil
}

// CloseShort closes a short position (Buy)
func (t *AlpacaTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if quantity <= 0 {
		logger.Infof("[Alpaca] Closing all Short position for %s", symbol)
		pos, err := t.client.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position for close: %w", err)
		}

		qtyStr := pos.Qty.String()
		qtyDec, _ := decimal.NewFromString(qtyStr)
		qtyDec = qtyDec.Abs() // Ensure positive qty for order

		if pos.Side != "short" {
			return map[string]interface{}{"status": "CLOSED", "info": "No short position found"}, nil
		}

		req := alpaca.PlaceOrderRequest{
			Symbol:      symbol,
			Qty:         &qtyDec,
			Side:        alpaca.Buy, // Buy to close Short
			Type:        alpaca.Market,
			TimeInForce: alpaca.GTC,
		}

		order, err := t.client.PlaceOrder(req)
		if err != nil {
			return nil, fmt.Errorf("Alpaca CloseAllShort failed: %w", err)
		}
		return map[string]interface{}{"orderId": order.ID, "status": "NEW"}, nil
	}

	qtyDec := decimal.NewFromFloat(quantity)
	req := alpaca.PlaceOrderRequest{
		Symbol:      symbol,
		Qty:         &qtyDec,
		Side:        alpaca.Buy, // Buy to close Short
		Type:        alpaca.Market,
		TimeInForce: alpaca.GTC,
	}

	logger.Infof("[Alpaca] Closing Short %s Qty: %.4f", symbol, quantity)
	order, err := t.client.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("Alpaca CloseShort failed: %w", err)
	}

	return map[string]interface{}{
		"orderId": order.ID,
		"status":  "NEW",
	}, nil
}

// SetLeverage (Not directly applicable to Alpaca API in the same way as Crypto Futures)
func (t *AlpacaTrader) SetLeverage(symbol string, leverage int) error {
	// Alpaca leverage is determined by account type and buying power, not per-symbol setting
	return nil
}

// SetMarginMode (Not applicable)
func (t *AlpacaTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

// GetMarketPrice retrieves current market price
func (t *AlpacaTrader) GetMarketPrice(symbol string) (float64, error) {
	// Use GetLatestTrade or Snapshot
	trade, err := t.mdClient.GetLatestTrade(symbol, marketdata.GetLatestTradeRequest{})
	if err != nil {
		return 0, fmt.Errorf("failed to get market price: %w", err)
	}
	return trade.Price, nil
}

// SetStopLoss sets a stop loss order
// Alpaca supports Bracket Orders, but we can also place separate Stop orders
func (t *AlpacaTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := alpaca.Sell
	if positionSide == "SHORT" {
		side = alpaca.Buy
	}

	qtyDec := decimal.NewFromFloat(quantity)
	stopDec := decimal.NewFromFloat(stopPrice)

	req := alpaca.PlaceOrderRequest{
		Symbol:      symbol,
		Qty:         &qtyDec,
		Side:        side,
		Type:        alpaca.Stop,
		StopPrice:   &stopDec,
		TimeInForce: alpaca.GTC,
	}

	_, err := t.client.PlaceOrder(req)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}
	return nil
}

// SetTakeProfit sets a take profit order
func (t *AlpacaTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := alpaca.Sell
	if positionSide == "SHORT" {
		side = alpaca.Buy
	}

	qtyDec := decimal.NewFromFloat(quantity)
	limitDec := decimal.NewFromFloat(takeProfitPrice)

	req := alpaca.PlaceOrderRequest{
		Symbol:      symbol,
		Qty:         &qtyDec,
		Side:        side,
		Type:        alpaca.Limit,
		LimitPrice:  &limitDec,
		TimeInForce: alpaca.GTC,
	}

	_, err := t.client.PlaceOrder(req)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}
	return nil
}

// CancelStopLossOrders (Implementation simplified for now)
func (t *AlpacaTrader) CancelStopLossOrders(symbol string) error {
	return t.CancelAllOrders(symbol) // For now, just cancel all opens as logic is complex to find specific SL
}

// CancelTakeProfitOrders
func (t *AlpacaTrader) CancelTakeProfitOrders(symbol string) error {
	return t.CancelAllOrders(symbol)
}

// CancelAllOrders
func (t *AlpacaTrader) CancelAllOrders(symbol string) error {
	// err := t.client.CancelOrders() // Limitation: Cancels ALL orders for account usually, need to check if SDK supports symbol filter
	// Alpaca Go SDK CancelOrders() cancels ALL open orders.
	// To cancel by symbol, we need to ListOrders and cancel individually.

	status := "open"
	orders, err := t.client.GetOrders(alpaca.GetOrdersRequest{
		Status:  status,
		Symbols: []string{symbol},
	})
	if err != nil {
		return err
	}

	for _, o := range orders {
		_ = t.client.CancelOrder(o.ID)
	}
	return nil
}

// CancelStopOrders
func (t *AlpacaTrader) CancelStopOrders(symbol string) error {
	return t.CancelAllOrders(symbol)
}

// FormatQuantity
func (t *AlpacaTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return fmt.Sprintf("%.9f", quantity), nil // Alpaca handles up to 9 decimals for fractional
}

// GetOrderStatus
func (t *AlpacaTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	order, err := t.client.GetOrder(orderID)
	if err != nil {
		return nil, err
	}

	execQty, _ := order.FilledQty.Float64()
	avgPrice, _ := order.FilledAvgPrice.Float64()

	return map[string]interface{}{
		"status":      order.Status,
		"executedQty": execQty,
		"avgPrice":    avgPrice,
	}, nil
}

// GetClosedPnL (Placeholder)
func (t *AlpacaTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return []ClosedPnLRecord{}, nil
}

// GetOpenOrders
func (t *AlpacaTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	status := "open"
	orders, err := t.client.GetOrders(alpaca.GetOrdersRequest{
		Status:  status,
		Symbols: []string{symbol},
	})
	if err != nil {
		return nil, err
	}

	var result []OpenOrder
	for _, o := range orders {
		qty, _ := o.Qty.Float64()
		limitPrice := 0.0
		if o.LimitPrice != nil {
			limitPrice, _ = o.LimitPrice.Float64()
		}
		stopPrice := 0.0
		if o.StopPrice != nil {
			stopPrice, _ = o.StopPrice.Float64()
		}

		positionSide := "LONG" // Default to long logic for basic orders
		if o.Side == alpaca.Buy {
			positionSide = "LONG" // Buy order to Open Long or Close Short
		} else {
			positionSide = "SHORT" // Sell order
		}

		result = append(result, OpenOrder{
			OrderID:      o.ID,
			Symbol:       o.Symbol,
			Side:         string(o.Side),
			PositionSide: positionSide,
			Type:         string(o.Type),
			Price:        limitPrice,
			StopPrice:    stopPrice,
			Quantity:     qty,
			Status:       o.Status,
		})
	}
	return result, nil
}
