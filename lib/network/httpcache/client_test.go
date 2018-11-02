package httpcache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	a := NewMemCacheAdapter(10)
	a.Set("http://foo?bar=1", &Response{
		Value:      []byte("value 1"),
		StatusCode: 200,
	}, nil)
	a.Set("http://foo.bar?bar=1&foo=1", &Response{
		Value:      []byte("value 2"),
		StatusCode: 200,
	}, nil)

	c, err := NewClient(
		WithAdapter(a),
		WithStatusCode(404, 1*time.Minute),
	)
	require.NoError(t, err)

	cnt := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/404" {
			w.WriteHeader(404)
		}

		w.Write([]byte(fmt.Sprintf("new value:%v", cnt)))
	})

	handler := c.Middleware(testHandler)

	type TestCase struct {
		name   string
		url    string
		method string
		body   string
		code   int
	}
	tests := []TestCase{
		{
			name:   "return resp (cached)",
			url:    "http://foo?bar=1",
			method: "GET",
			body:   "value 1",
			code:   200,
		},
		{
			name:   "return resp",
			url:    "http://foo?bar=2",
			method: "GET",
			body:   "new value:2",
			code:   200,
		},
		{
			name:   "return resp with params (cached)",
			url:    "http://foo.bar?bar=1&foo=1",
			method: "GET",
			body:   "value 2",
			code:   200,
		},
		{
			name:   "return 404",
			url:    "/404",
			method: "GET",
			body:   "new value:4",
			code:   404,
		},
		{
			name:   "return 404 (cached)",
			url:    "/404",
			method: "GET",
			body:   "new value:4",
			code:   404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cnt++

			r, err := http.NewRequest(tt.method, tt.url, nil)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			require.Equal(t, w.Code, tt.code)
			require.Equal(t, w.Body.String(), tt.body)
		})
	}
}
