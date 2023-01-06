// Copyright 2021-2023 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connect_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshcarp/connect-no/internal/assert"
)

func TestHandler_ServeHTTP(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.Handle(pingv1connect_test.NewPingServiceHandler(
		successPingServer{},
	))
	const pingProcedure = "/" + pingv1connect_test.PingServiceName + "/Ping"
	server := httptest.NewServer(mux)
	client := server.Client()
	t.Cleanup(func() {
		server.Close()
	})

	t.Run("method_not_allowed", func(t *testing.T) {
		t.Parallel()
		request, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			server.URL+pingProcedure,
			strings.NewReader(""),
		)
		assert.Nil(t, err)
		resp, err := client.Do(request)
		assert.Nil(t, err)
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusMethodNotAllowed)
		assert.Equal(t, resp.Header.Get("Allow"), http.MethodPost)
	})

	t.Run("unsupported_content_type", func(t *testing.T) {
		t.Parallel()
		request, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			server.URL+pingProcedure,
			strings.NewReader("{}"),
		)
		assert.Nil(t, err)
		request.Header.Set("Content-Type", "application/x-custom-json")
		resp, err := client.Do(request)
		assert.Nil(t, err)
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusUnsupportedMediaType)
		assert.Equal(t, resp.Header.Get("Accept-Post"), strings.Join([]string{
			"application/grpc",
			"application/grpc+json",
			"application/grpc+json; charset=utf-8",
			"application/grpc+proto",
			"application/grpc-web",
			"application/grpc-web+json",
			"application/grpc-web+json; charset=utf-8",
			"application/grpc-web+proto",
			"application/json",
			"application/json; charset=utf-8",
			"application/proto",
		}, ", "))
	})

	t.Run("charset_in_content_type_header", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			server.URL+pingProcedure,
			strings.NewReader("{}"),
		)
		assert.Nil(t, err)
		req.Header.Set("Content-Type", "application/json;Charset=utf-8")
		resp, err := client.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusOK)
	})

	t.Run("unsupported_charset", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			server.URL+pingProcedure,
			strings.NewReader("{}"),
		)
		assert.Nil(t, err)
		req.Header.Set("Content-Type", "application/json; charset=shift-jis")
		resp, err := client.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusUnsupportedMediaType)
	})

	t.Run("unsupported_content_encoding", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			server.URL+pingProcedure,
			strings.NewReader("{}"),
		)
		assert.Nil(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "invalid")
		resp, err := client.Do(req)
		assert.Nil(t, err)
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusNotFound)

		type errorMessage struct {
			Code, Message string
		}
		var message errorMessage
		err = json.NewDecoder(resp.Body).Decode(&message)
		assert.Nil(t, err)
		assert.Equal(t, message.Message, `unknown compression "invalid": supported encodings are gzip`)
		assert.Equal(t, message.Code, connect.CodeUnimplemented.String())
	})
}

type successPingServer struct {
	pingv1connect_test.UnimplementedPingServiceHandler
}

func (successPingServer) Ping(context.Context, *connect.Request[pingv1_test.PingRequest]) (*connect.Response[pingv1_test.PingResponse], error) {
	return &connect.Response[pingv1_test.PingResponse]{}, nil
}
