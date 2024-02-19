package utils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestMarshalUnmarshalNested(t *testing.T) {
	tests := []struct {
		name         string
		request      Request[[]byte]
		expectedJSON string
	}{
		{
			name: "simple request",
			request: Request[[]byte]{
				Call:     "1",
				Function: "Println",
				Args:     [][]byte{[]byte(`"Hello, world!"`)},
			},
			expectedJSON: `{"call":"1","function":"Println","args":["IkhlbGxvLCB3b3JsZCEi"]}`,
		},
		{
			name: "complex request",
			request: Request[[]byte]{
				Call:     "1-2-3",
				Function: "Add",
				Args:     [][]byte{[]byte("1"), []byte("2"), []byte(`{"asdf":1}`)},
			},
			expectedJSON: `{"call":"1-2-3","function":"Add","args":["MQ==","Mg==","eyJhc2RmIjoxfQ=="]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaled, err := tt.request.Marshal(json.Marshal)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expectedJSON, string(marshaled))

			var unmarshaledRequest Request[[]byte]
			err = unmarshaledRequest.Unmarshal(marshaled, json.Unmarshal)
			assert.NoError(t, err)
			assert.Equal(t, tt.request, unmarshaledRequest)
		})
	}
}

func TestRequestMarshalUnmarshalFlat(t *testing.T) {
	tests := []struct {
		name         string
		request      Request[json.RawMessage]
		expectedJSON string
	}{
		{
			name: "simple request",
			request: Request[json.RawMessage]{
				Call:     "1",
				Function: "Println",
				Args:     []json.RawMessage{json.RawMessage(`"Hello, world!"`)},
			},
			expectedJSON: `{"call":"1","function":"Println","args":["Hello, world!"]}`,
		},
		{
			name: "complex request",
			request: Request[json.RawMessage]{
				Call:     "1-2-3",
				Function: "Add",
				Args:     []json.RawMessage{json.RawMessage("1"), json.RawMessage("2"), json.RawMessage(`{"asdf":1}`)},
			},
			expectedJSON: `{"call":"1-2-3","function":"Add","args":[1,2,{"asdf":1}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaled, err := tt.request.Marshal(
				func(v any) (json.RawMessage, error) {
					b, err := json.Marshal(v)
					if err != nil {
						return nil, err
					}

					return json.RawMessage(b), nil
				},
			)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expectedJSON, string(marshaled))

			var unmarshaledRequest Request[json.RawMessage]
			err = unmarshaledRequest.Unmarshal(
				marshaled,
				func(data json.RawMessage, v any) error {
					return json.Unmarshal([]byte(data), v)
				},
			)
			assert.NoError(t, err)
			assert.Equal(t, tt.request, unmarshaledRequest)
		})
	}
}

func TestResponseMarshalUnmarshalNested(t *testing.T) {
	tests := []struct {
		name         string
		response     Response[[]byte]
		expectedJSON string
	}{
		{
			name: "simple response",
			response: Response[[]byte]{
				Call:  "1",
				Value: []byte(`true`),
				Err:   "",
			},
			expectedJSON: `{"call":"1","value":"dHJ1ZQ==","err":""}`,
		},
		{
			name: "complex response",
			response: Response[[]byte]{
				Call:  "1",
				Value: []byte(`{"a":1,"b":{"c":"test"}}`),
				Err:   "test error",
			},
			expectedJSON: `{"call":"1","value":"eyJhIjoxLCJiIjp7ImMiOiJ0ZXN0In19","err":"test error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaled, err := tt.response.Marshal(json.Marshal)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expectedJSON, string(marshaled))

			var unmarshaledResponse Response[[]byte]
			err = unmarshaledResponse.Unmarshal(marshaled, json.Unmarshal)
			assert.NoError(t, err)
			assert.Equal(t, tt.response, unmarshaledResponse)
		})
	}
}

func TestResponseMarshalUnmarshalFlat(t *testing.T) {
	tests := []struct {
		name         string
		response     Response[json.RawMessage]
		expectedJSON string
	}{
		{
			name: "simple response",
			response: Response[json.RawMessage]{
				Call:  "1",
				Value: json.RawMessage(`true`),
				Err:   "",
			},
			expectedJSON: `{"call":"1","value":true,"err":""}`,
		},
		{
			name: "complex response",
			response: Response[json.RawMessage]{
				Call:  "1",
				Value: json.RawMessage(`{"a":1,"b":{"c":"test"}}`),
				Err:   "test error",
			},
			expectedJSON: `{"call":"1","value":{"a":1,"b":{"c":"test"}},"err":"test error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaled, err := tt.response.Marshal(
				func(v any) (json.RawMessage, error) {
					b, err := json.Marshal(v)
					if err != nil {
						return nil, err
					}

					return json.RawMessage(b), nil
				},
			)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expectedJSON, string(marshaled))

			var unmarshaledResponse Response[json.RawMessage]
			err = unmarshaledResponse.Unmarshal(
				marshaled,
				func(data json.RawMessage, v any) error {
					return json.Unmarshal([]byte(data), v)
				},
			)
			assert.NoError(t, err)
			assert.Equal(t, tt.response, unmarshaledResponse)
		})
	}
}
