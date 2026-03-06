package hyperliquid

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock implementation of subscriptable interface for testing
type mockSubscriptable struct {
	key string
}

func (m mockSubscriptable) Key() string {
	return m.key
}

func TestUniqSubscriber(t *testing.T) {
	t.Run("NewUniqSubscriber", func(t *testing.T) {
		tests := []struct {
			name             string
			id               string
			payload          subscriptable
			subscriberFunc   func(subscriptable)
			unsubscriberFunc func(subscriptable)
			wantID           string
			wantCount        int64
			wantPayloadKey   string
		}{
			{
				name:             "create new subscriber with valid data",
				id:               "trades:BTC",
				payload:          mockSubscriptable{key: "trades:BTC"},
				subscriberFunc:   func(subscriptable) {},
				unsubscriberFunc: func(subscriptable) {},
				wantID:           "trades:BTC",
				wantCount:        0,
				wantPayloadKey:   "trades:BTC",
			},
			{
				name:             "create new subscriber with empty id",
				id:               "",
				payload:          mockSubscriptable{key: ""},
				subscriberFunc:   func(subscriptable) {},
				unsubscriberFunc: func(subscriptable) {},
				wantID:           "",
				wantCount:        0,
				wantPayloadKey:   "",
			},
			{
				name:             "create new subscriber with nil functions",
				id:               "candles:ETH",
				payload:          mockSubscriptable{key: "candles:ETH"},
				subscriberFunc:   nil,
				unsubscriberFunc: nil,
				wantID:           "candles:ETH",
				wantCount:        0,
				wantPayloadKey:   "candles:ETH",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				subscriber := newUniqSubscriber(
					tt.id,
					tt.payload,
					tt.subscriberFunc,
					tt.unsubscriberFunc,
				)

				assert.NotNil(t, subscriber)
				assert.Equal(t, tt.wantID, subscriber.id)
				assert.Equal(t, tt.wantCount, subscriber.count)
				assert.NotNil(t, subscriber.subscribers)
				assert.Empty(t, subscriber.subscribers)
				assert.Equal(t, tt.wantPayloadKey, subscriber.subscriptionPayload.Key())
			})
		}
	})

	t.Run("Subscriptions", func(t *testing.T) {
		// timeline-type test table.
		tests := []struct {
			name                       string
			op                         string
			subscriberID               string
			callback                   callback
			wantCount                  int64
			wantSubscriberLen          int
			wantSubscribeCalledTimes   int
			wantUnsubscribeCalledTimes int
		}{
			{
				name:                       "subscription already exists",
				op:                         "subscribe",
				subscriberID:               "sub1",
				callback:                   func(any) {},
				wantCount:                  1,
				wantSubscriberLen:          1,
				wantSubscribeCalledTimes:   1,
				wantUnsubscribeCalledTimes: 0,
			},
			{
				name:                       "new subscription",
				op:                         "subscribe",
				subscriberID:               "sub2",
				callback:                   func(any) {},
				wantCount:                  2,
				wantSubscriberLen:          2,
				wantSubscribeCalledTimes:   1,
				wantUnsubscribeCalledTimes: 0,
			},
			{
				name:                       "unsubscribe should not trigger unsubscribe function if multiple subscribers exist",
				op:                         "unsubscribe",
				subscriberID:               "sub2",
				callback:                   func(any) {},
				wantCount:                  1,
				wantSubscriberLen:          1,
				wantSubscribeCalledTimes:   1,
				wantUnsubscribeCalledTimes: 0,
			},
			{
				name:                       "unsubscribe should trigger unsubscribe function if no other subscribers exist",
				op:                         "unsubscribe",
				subscriberID:               "sub1",
				callback:                   func(any) {},
				wantCount:                  0,
				wantSubscriberLen:          0,
				wantSubscribeCalledTimes:   1,
				wantUnsubscribeCalledTimes: 1,
			},
		}

		haveSubscribeCalledTimes := 0
		haveUnsubscribeCalledTimes := 0
		subscriberFunc := func(subscriptable) { haveSubscribeCalledTimes++ }
		unsubscriberFunc := func(subscriptable) { haveUnsubscribeCalledTimes++ }

		payload := mockSubscriptable{key: "test"}
		subscriber := newUniqSubscriber("test", payload, subscriberFunc, unsubscriberFunc)
		// set initial state
		subscriber.subscribe("sub1", func(any) {})
		assert.Equal(t, 1, haveSubscribeCalledTimes)
		assert.Equal(t, 0, haveUnsubscribeCalledTimes)

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				switch tt.op {
				case "subscribe":
					subscriber.subscribe(tt.subscriberID, tt.callback)
				case "unsubscribe":
					subscriber.unsubscribe(tt.subscriberID)
				default:
					t.Fatalf("unknown operation: %s", tt.op)
				}

				assert.Equal(t, tt.wantCount, subscriber.count)
				assert.Len(t, subscriber.subscribers, tt.wantSubscriberLen)

				if tt.op == "subscribe" {
					assert.Contains(t, subscriber.subscribers, tt.subscriberID)
				} else {
					assert.NotContains(t, subscriber.subscribers, tt.subscriberID)
				}
				assert.Equal(t, tt.wantSubscribeCalledTimes, haveSubscribeCalledTimes)
				assert.Equal(t, tt.wantUnsubscribeCalledTimes, haveUnsubscribeCalledTimes)
			})
		}
	})

	t.Run("Dispatch", func(t *testing.T) {
		tests := []struct {
			name           string
			subscribers    map[string]callback
			data           any
			wantCallCounts map[string]int
		}{
			{
				name:           "dispatch to no subscribers",
				subscribers:    map[string]callback{},
				data:           "test data",
				wantCallCounts: map[string]int{},
			},
			{
				name: "dispatch to single subscriber",
				subscribers: map[string]callback{
					"sub1": nil, // will be set in test
				},
				data: "test data",
				wantCallCounts: map[string]int{
					"sub1": 1,
				},
			},
			{
				name: "dispatch to multiple subscribers",
				subscribers: map[string]callback{
					"sub1": nil, // will be set in test
					"sub2": nil, // will be set in test
					"sub3": nil, // will be set in test
				},
				data: "test data",
				wantCallCounts: map[string]int{
					"sub1": 1,
					"sub2": 1,
					"sub3": 1,
				},
			},
			{
				name: "dispatch with nil data",
				subscribers: map[string]callback{
					"sub1": nil, // will be set in test
				},
				data: nil,
				wantCallCounts: map[string]int{
					"sub1": 1,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				payload := mockSubscriptable{key: "test"}
				subscriber := newUniqSubscriber(
					"test",
					payload,
					func(subscriptable) {},
					func(subscriptable) {},
				)

				callCounts := make(map[string]int)
				var mu sync.Mutex

				// Create callbacks that track call counts
				for id := range tt.subscribers {
					tt.subscribers[id] = func(data any) {
						mu.Lock()
						callCounts[id]++
						mu.Unlock()
						assert.Equal(t, tt.data, data)
					}
				}

				subscriber.subscribers = tt.subscribers
				subscriber.count = int64(len(tt.subscribers))

				subscriber.dispatch(tt.data)

				mu.Lock()
				assert.Equal(t, tt.wantCallCounts, callCounts)
				mu.Unlock()
			})
		}
	})

	t.Run("Clear", func(t *testing.T) {
		tests := []struct {
			name                string
			initialSubscribers  map[string]callback
			shouldCallUnsubFunc bool
		}{
			{
				name:                "clear empty subscribers",
				initialSubscribers:  map[string]callback{},
				shouldCallUnsubFunc: true,
			},
			{
				name: "clear single subscriber",
				initialSubscribers: map[string]callback{
					"sub1": func(any) {},
				},
				shouldCallUnsubFunc: true,
			},
			{
				name: "clear multiple subscribers",
				initialSubscribers: map[string]callback{
					"sub1": func(any) {},
					"sub2": func(any) {},
					"sub3": func(any) {},
				},
				shouldCallUnsubFunc: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				unsubFuncCalled := false
				subscriberFunc := func(subscriptable) {}
				unsubscriberFunc := func(subscriptable) { unsubFuncCalled = true }

				payload := mockSubscriptable{key: "test"}
				subscriber := newUniqSubscriber("test", payload, subscriberFunc, unsubscriberFunc)

				// Set initial state
				subscriber.subscribers = make(map[string]callback)
				for id, cb := range tt.initialSubscribers {
					subscriber.subscribers[id] = cb
				}
				subscriber.count = int64(len(tt.initialSubscribers))

				subscriber.clear()

				assert.Equal(t, int64(0), subscriber.count)
				assert.Empty(t, subscriber.subscribers)
				assert.Equal(t, tt.shouldCallUnsubFunc, unsubFuncCalled)
			})
		}
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		t.Run("concurrent subscribe and unsubscribe", func(t *testing.T) {
			payload := mockSubscriptable{key: "test"}
			subscriber := newUniqSubscriber(
				"test",
				payload,
				func(subscriptable) {},
				func(subscriptable) {},
			)

			var wg sync.WaitGroup
			numGoroutines := 10

			// Concurrent subscribes
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					subscriber.subscribe(string(rune('A'+id)), func(any) {})
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						subscriber.unsubscribe(string(rune('A' + id)))
					}(id)
				}(i)
			}

			wg.Wait()

			// Verify final state is consistent
			actualLen := len(subscriber.subscribers)
			count := subscriber.count
			assert.Equal(t, actualLen, 0)
			assert.True(t, count == 0)
		})

		t.Run("concurrent dispatch and subscribe", func(t *testing.T) {
			payload := mockSubscriptable{key: "test"}
			subscriber := newUniqSubscriber(
				"test",
				payload,
				func(subscriptable) {},
				func(subscriptable) {},
			)

			var wg sync.WaitGroup
			var callCount int64
			var mu sync.Mutex

			// Add initial subscriber
			subscriber.subscribe("initial", func(any) {
				mu.Lock()
				callCount++
				mu.Unlock()
			})

			// Concurrent dispatches
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					subscriber.dispatch("test")
				}()
			}

			// Concurrent subscribes
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					subscriber.subscribe(string(rune('A'+id)), func(any) {
						mu.Lock()
						callCount++
						mu.Unlock()
					})
				}(i)
			}

			wg.Wait()

			mu.Lock()
			finalCallCount := callCount
			mu.Unlock()

			// Should have at least 5 calls (from initial subscriber)
			assert.True(t, finalCallCount >= 5, "should have received at least 5 calls")
		})
	})
	t.Run("SubscriptionPayloadPersistence", func(t *testing.T) {
		t.Run("subscription payload should be passed to subscriber function", func(t *testing.T) {
			var receivedPayload subscriptable
			subscriberFunc := func(payload subscriptable) {
				receivedPayload = payload
			}
			unsubscriberFunc := func(subscriptable) {}

			wantPayload := mockSubscriptable{key: "test-payload"}
			subscriber := newUniqSubscriber(
				"test",
				wantPayload,
				subscriberFunc,
				unsubscriberFunc,
			)

			// First subscription should trigger the subscriber function
			subscriber.subscribe("sub1", func(any) {})

			assert.Equal(t, wantPayload, receivedPayload)
		})

		t.Run("subscription payload should be passed to unsubscriber function", func(t *testing.T) {
			var receivedPayload subscriptable
			subscriberFunc := func(subscriptable) {}
			unsubscriberFunc := func(payload subscriptable) {
				receivedPayload = payload
			}

			wantPayload := mockSubscriptable{key: "test-payload"}
			subscriber := newUniqSubscriber(
				"test",
				wantPayload,
				subscriberFunc,
				unsubscriberFunc,
			)

			// Add and remove subscriber to trigger unsubscriber function
			subscriber.subscribe("sub1", func(any) {})
			subscriber.unsubscribe("sub1")

			assert.Equal(t, wantPayload, receivedPayload)
		})

		t.Run(
			"clear should pass subscription payload to unsubscriber function",
			func(t *testing.T) {
				var receivedPayload subscriptable
				subscriberFunc := func(subscriptable) {}
				unsubscriberFunc := func(payload subscriptable) {
					receivedPayload = payload
				}

				wantPayload := mockSubscriptable{key: "test-payload"}
				subscriber := newUniqSubscriber(
					"test",
					wantPayload,
					subscriberFunc,
					unsubscriberFunc,
				)

				// Add subscribers and clear
				subscriber.subscribe("sub1", func(any) {})
				subscriber.subscribe("sub2", func(any) {})
				subscriber.clear()

				assert.Equal(t, wantPayload, receivedPayload)
			},
		)
	})
}
