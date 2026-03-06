package hyperliquid

import (
	"fmt"
)

type TradesSubscriptionParams struct {
	Coin string
}

func (w *WebsocketClient) Trades(
	params TradesSubscriptionParams,
	callback func([]Trade, error),
) (*Subscription, error) {
	remotePayload := remoteTradesSubscriptionPayload{
		Type: ChannelTrades,
		Coin: params.Coin,
	}

	return w.subscribe(remotePayload, func(msg any) {
		trades, ok := msg.(Trades)
		if !ok {
			callback(nil, fmt.Errorf("SubscribeToTrades invalid message type"))
			return
		}

		callback(trades, nil)
	})
}
