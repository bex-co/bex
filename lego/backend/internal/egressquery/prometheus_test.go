/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package egressquery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInstantSumsFiniteVectorValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("query"); got != "sum(test_total)" {
			t.Errorf("query = %q", got)
		}
		_, _ = writer.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[1,"2.5"]},{"value":[1,"3.5"]}]}}`))
	}))
	defer server.Close()

	got, err := Instant(context.Background(), server.Client(), server.URL, "sum(test_total)", time.Unix(1, 0))
	if err != nil || got != 6 {
		t.Fatalf("Instant = (%v, %v), want (6, nil)", got, err)
	}
}

func TestInstantTreatsExplicitEmptyVectorAsZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()
	got, err := Instant(context.Background(), server.Client(), server.URL, "empty", time.Now())
	if err != nil || got != 0 {
		t.Fatalf("Instant empty vector = (%v, %v), want (0, nil)", got, err)
	}
}

func TestInstantRejectsLossyOrMalformedSamples(t *testing.T) {
	tests := map[string]string{
		"nan":            `{"status":"success","data":{"resultType":"vector","result":[{"value":[1,"NaN"]}]}}`,
		"infinite":       `{"status":"success","data":{"resultType":"vector","result":[{"value":[1,"+Inf"]}]}}`,
		"negative":       `{"status":"success","data":{"resultType":"vector","result":[{"value":[1,"-1"]}]}}`,
		"sum overflow":   `{"status":"success","data":{"resultType":"vector","result":[{"value":[1,"1e308"]},{"value":[1,"1e308"]}]}}`,
		"not a number":   `{"status":"success","data":{"resultType":"vector","result":[{"value":[1,"broken"]}]}}`,
		"wrong shape":    `{"status":"success","data":{"resultType":"vector","result":[{"value":[1]}]}}`,
		"bad timestamp":  `{"status":"success","data":{"resultType":"vector","result":[{"value":["now","1"]}]}}`,
		"wrong type":     `{"status":"success","data":{"resultType":"scalar","result":[]}}`,
		"missing data":   `{"status":"success"}`,
		"missing result": `{"status":"success","data":{"resultType":"vector"}}`,
		"null result":    `{"status":"success","data":{"resultType":"vector","result":null}}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(body))
			}))
			defer server.Close()
			if _, err := Instant(context.Background(), server.Client(), server.URL, "lossy", time.Now()); err == nil {
				t.Fatal("Instant accepted a lossy or malformed sample")
			}
		})
	}
}

func TestInstantRejectsPrometheusFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"status":"error","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()
	if _, err := Instant(context.Background(), server.Client(), server.URL, "bad", time.Now()); err == nil {
		t.Fatal("Instant accepted an error response")
	}
}

func TestRegexEscape(t *testing.T) {
	if got, want := RegexEscape(`app.v1+blue`), `app\.v1\+blue`; got != want {
		t.Fatalf("RegexEscape = %q, want %q", got, want)
	}
}
