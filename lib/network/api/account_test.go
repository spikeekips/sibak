package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"

	"boscoin.io/sebak/lib/block"
	"boscoin.io/sebak/lib/common/observer"
	"boscoin.io/sebak/lib/error"
	"boscoin.io/sebak/lib/network/httputils"
	"github.com/stellar/go/keypair"
	"github.com/stretchr/testify/require"
)

func TestGetAccountHandler(t *testing.T) {
	ts, storage, err := prepareAPIServer()
	require.Nil(t, err)
	defer storage.Close()
	defer ts.Close()
	// Make Dummy BlockAccount
	ba := block.TestMakeBlockAccount()
	fmt.Println(ba)
	ba.MustSave(storage)
	{
		// Do a Request
		url := strings.Replace(GetAccountHandlerPattern, "{id}", ba.Address, -1)
		respBody, err := request(ts, url, false)
		require.Nil(t, err)
		defer respBody.Close()
		reader := bufio.NewReader(respBody)

		readByte, err := ioutil.ReadAll(reader)
		require.Nil(t, err)
		recv := make(map[string]interface{})
		var ba block.BlockAccount
		json.Unmarshal(readByte, &recv)
		json.Unmarshal(readByte, &ba)
		fmt.Println(ba)
		require.Equal(t, ba.Address, recv["address"], "address is not same")
	}

	{ // unknown address
		unknownKey, _ := keypair.Random()
		url := strings.Replace(GetAccountHandlerPattern, "{id}", unknownKey.Address(), -1)
		req, _ := http.NewRequest("GET", ts.URL+url, nil)
		resp, err := ts.Client().Do(req)
		require.Nil(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
}

func TestGetAccountHandlerStream(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	ts, storage, err := prepareAPIServer()
	require.Nil(t, err)
	defer storage.Close()
	defer ts.Close()
	ba := block.TestMakeBlockAccount()
	ba.MustSave(storage)
	require.Nil(t, err)

	key := ba.Address

	// Wait until request registered to observer
	{
		go func() {
			for {
				observer.BlockAccountObserver.RLock()
				if len(observer.BlockAccountObserver.Callbacks) > 0 {
					observer.BlockAccountObserver.RUnlock()
					break
				}
				observer.BlockAccountObserver.RUnlock()
			}
			ba.MustSave(storage)
			wg.Done()
		}()
	}

	// Do a Request
	var reader *bufio.Reader
	{
		url := strings.Replace(GetAccountHandlerPattern, "{id}", key, -1)
		respBody, err := request(ts, url, true)
		require.Nil(t, err)
		defer respBody.Close()
		reader = bufio.NewReader(respBody)
	}

	// Check the output
	{
		line, err := reader.ReadBytes('\n')
		require.Nil(t, err)
		recv := make(map[string]interface{})
		json.Unmarshal(line, &recv)
		require.Equal(t, key, recv["address"], "address is not same")
	}
	wg.Wait()
}

// Test that getting an inexisting account returns an error
func TestGetNonExistentAccountHandler(t *testing.T) {

	ts, storage, err := prepareAPIServer()
	require.Nil(t, err)
	defer storage.Close()
	defer ts.Close()

	p := httputils.NewErrorProblem(errors.ErrorBlockAccountDoesNotExists, httputils.StatusCode(errors.ErrorBlockAccountDoesNotExists))

	{
		// Do a Request
		kp, _ := keypair.Random()
		url := strings.Replace(GetAccountHandlerPattern, "{id}", kp.Address(), -1)
		respBody, err := request(ts, url, false)
		require.Nil(t, err)
		reader := bufio.NewReader(respBody)
		readByte, err := ioutil.ReadAll(reader)
		require.Nil(t, err)
		pByte, err := json.Marshal(p)
		require.Nil(t, err)
		require.Equal(t, pByte, readByte)
	}
}

func TestUnmarshal(t *testing.T) {
	b := B{
		"abc": "bulabula",
		"edf": "bulabula",
	}
	encoded, _ := json.Marshal(b)

	var a A
	json.Unmarshal(encoded, &a)

	fmt.Println(a)
}

type A struct {
	ABC string
	EDF string
}

type B map[string]interface{}
