package hyperliquid

import "fmt"

type WebData2SubscriptionParams struct {
	User string
}

func (w *WebsocketClient) WebData2(
	params WebData2SubscriptionParams,
	callback func(WebData2, error),
) (*Subscription, error) {
	payload := remoteWebData2SubscriptionPayload{
		Type: ChannelWebData2,
		User: params.User,
	}

	return w.subscribe(payload, func(msg any) {
		webdata2, ok := msg.(WebData2)
		if !ok {
			callback(WebData2{}, fmt.Errorf("invalid message type"))
			return
		}

		callback(webdata2, nil)
	})
}
