package api

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"boscoin.io/sebak/lib/block"
	"boscoin.io/sebak/lib/common"
	"github.com/GianlucaGuarini/go-observable"
	"github.com/stretchr/testify/require"
)

func TestAPIStreamRun(t *testing.T) {
	tests := []struct {
		name       string
		events     []string
		makeStream func(http.ResponseWriter, *http.Request) *EventStream
		trigger    func(*observable.Observable)
		respFunc   func(testing.TB, *http.Response)
	}{
		{
			"default",
			[]string{"test1"},
			func(w http.ResponseWriter, r *http.Request) *EventStream {
				es := NewDefaultEventStream(w, r)
				return es
			},
			func(ob *observable.Observable) {
				ob.Trigger("test1", block.NewBlockAccount("hello", 100))
			},
			func(t testing.TB, res *http.Response) {
				s := bufio.NewScanner(res.Body)
				s.Scan()

				var ba block.BlockAccount
				common.MustUnmarshalJSON(s.Bytes(), &ba)
				require.Nil(t, s.Err())
				require.Equal(t, ba, *block.NewBlockAccount("hello", 100))
			},
		},
		{
			"renderFunc",
			[]string{"test1"},
			func(w http.ResponseWriter, r *http.Request) *EventStream {
				renderFunc := func(args ...interface{}) ([]byte, error) {
					s, ok := args[1].(*block.BlockAccount)
					if !ok {
						return nil, fmt.Errorf("this is not serializable")
					}
					return common.MustMarshalJSON(s), nil
				}
				es := NewEventStream(w, r, renderFunc, DefaultContentType)
				return es
			},
			func(ob *observable.Observable) {
				ob.Trigger("test1", block.NewBlockAccount("hello", 100))
			},
			func(t testing.TB, res *http.Response) {
				s := bufio.NewScanner(res.Body)
				s.Scan()

				var ba block.BlockAccount
				common.MustUnmarshalJSON(s.Bytes(), &ba)
				require.Nil(t, s.Err())
				require.Equal(t, ba, *block.NewBlockAccount("hello", 100))
			},
		},
		{
			"renderBeforeObservable",
			[]string{"test1"},
			func(w http.ResponseWriter, r *http.Request) *EventStream {
				es := NewDefaultEventStream(w, r)
				es.Render(block.NewBlockAccount("hello", 100))
				return es
			},
			nil, // no trigger
			func(t testing.TB, res *http.Response) {
				s := bufio.NewScanner(res.Body)
				s.Scan()

				var ba block.BlockAccount
				common.MustUnmarshalJSON(s.Bytes(), &ba)
				require.Nil(t, s.Err())
				require.Equal(t, ba, *block.NewBlockAccount("hello", 100))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ready := make(chan chan struct{})
			ob := observable.New()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				es := test.makeStream(w, r)
				run := es.Start(ob, test.events...)

				if test.trigger != nil {
					c := <-ready
					close(c)
				}

				run()
			}))
			defer ts.Close()

			if test.trigger != nil {
				go func() {
					c := make(chan struct{})
					ready <- c
					<-c
					test.trigger(ob)
				}()
			}

			req, err := http.NewRequest("GET", ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(req.Context())
			defer cancel()

			req = req.WithContext(ctx)

			res, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()

			test.respFunc(t, res)
		})
	}
}
