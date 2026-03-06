package hyperliquid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIResponse_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		want     *APIResponse[OrderResponse]
	}{
		{
			name:     "CreateOrderRequest Response",
			jsonData: `{"status":"ok","response":{"type":"order","data":{"statuses":[{"resting":{"oid":12345678901,"cloid":"0x00000000000000000000000000000000"}}]}}}`,
			want: &APIResponse[OrderResponse]{
				Status: "ok",
				Ok:     true,
				Type:   "order",
				Data: OrderResponse{
					Statuses: []OrderStatus{
						{
							Resting: &OrderStatusResting{
								Oid:      12345678901,
								ClientID: stringPtr("0x00000000000000000000000000000000"),
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &APIResponse[OrderResponse]{}
			err := res.UnmarshalJSON([]byte(tt.jsonData))
			require.NoError(t, err)
			assert.Equal(t, tt.want, res)
		})
	}
}
