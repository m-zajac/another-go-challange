package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

var (
	SimpleContentRequest, _ = http.NewRequest("GET", "/?offset=0&count=5", nil)
	OffsetContentRequest, _ = http.NewRequest("GET", "/?offset=5&count=5", nil)
)

func runDefaultServiceRequest(t *testing.T, r *http.Request) (status int, content []*ContentItem) {
	t.Helper()

	service, err := NewDefaultService()
	if err != nil {
		t.Fatalf("creating a service: %v", err)
	}

	return runRequest(t, service, r)
}

func runRequest(t *testing.T, service *Service, r *http.Request) (status int, content []*ContentItem) {
	t.Helper()

	handler := &Handler{service: service}

	srv := httptest.NewServer(handler)
	defer srv.Close()

	r.URL.Scheme = ""
	r.URL.Host = ""
	r.URL.RequestURI()
	u, _ := url.Parse(srv.URL + r.URL.String())
	r.URL = u

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("server returned error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}

	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(&content); err != nil {
		t.Fatalf("couldn't decode response: %v", err)
	}

	return http.StatusOK, content
}

func TestResponseCount(t *testing.T) {
	status, content := runDefaultServiceRequest(t, SimpleContentRequest)

	if status != http.StatusOK {
		t.Fatalf("got response status %d", status)
	}
	if len(content) != 5 {
		t.Fatalf("got %d items back, want 5", len(content))
	}
}

func TestResponseOrder(t *testing.T) {
	status, content := runDefaultServiceRequest(t, SimpleContentRequest)

	if status != http.StatusOK {
		t.Fatalf("got response status %d", status)
	}
	if len(content) != 5 {
		t.Fatalf("got %d items back, want 5", len(content))
	}

	for i, item := range content {
		if Provider(item.Source) != DefaultConfig[i%len(DefaultConfig)].Type {
			t.Errorf(
				"position %d: Got Provider %v instead of Provider %v",
				i, item.Source, DefaultConfig[i%len(DefaultConfig)].Type,
			)
		}
	}
}

func TestOffsetResponseOrder(t *testing.T) {
	status, content := runDefaultServiceRequest(t, OffsetContentRequest)

	if status != http.StatusOK {
		t.Fatalf("got response status %d", status)
	}
	if len(content) != 5 {
		t.Fatalf("got %d items back, want 5", len(content))
	}

	for j, item := range content {
		i := j + 5
		if Provider(item.Source) != DefaultConfig[i%len(DefaultConfig)].Type {
			t.Errorf(
				"position %d: Got Provider %v instead of Provider %v",
				i, item.Source, DefaultConfig[i%len(DefaultConfig)].Type,
			)
		}
	}
}

func TestRequestValidation(t *testing.T) {
	tests := map[string]struct {
		method     string
		target     string
		wantStatus int
	}{
		"invalid method post": {
			method:     http.MethodPost,
			target:     "/",
			wantStatus: http.StatusNotFound,
		},
		"invalid method delete": {
			method:     http.MethodDelete,
			target:     "/",
			wantStatus: http.StatusNotFound,
		},
		"empty parameters": {
			method:     http.MethodGet,
			target:     "/",
			wantStatus: http.StatusBadRequest,
		},
		"zero count": {
			method:     http.MethodGet,
			target:     "/?count=0",
			wantStatus: http.StatusBadRequest,
		},
		"invalid count": {
			method:     http.MethodGet,
			target:     "/?count=abc",
			wantStatus: http.StatusBadRequest,
		},
		"negative count": {
			method:     http.MethodGet,
			target:     "/?count=-5",
			wantStatus: http.StatusBadRequest,
		},
		"valid count": {
			method:     http.MethodGet,
			target:     "/?count=3",
			wantStatus: http.StatusOK,
		},
		"invalid offset": {
			method:     http.MethodGet,
			target:     "/?count=3&offset=abc",
			wantStatus: http.StatusBadRequest,
		},
		"negative offset": {
			method:     http.MethodGet,
			target:     "/?count=3&offset=-5",
			wantStatus: http.StatusBadRequest,
		},
		"valid offset - zero": {
			method:     http.MethodGet,
			target:     "/?count=3&offset=0",
			wantStatus: http.StatusOK,
		},
		"valid offset": {
			method:     http.MethodGet,
			target:     "/?count=3&offset=5",
			wantStatus: http.StatusOK,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, tc.target, nil)
			status, _ := runDefaultServiceRequest(t, req)
			if status != tc.wantStatus {
				t.Fatalf("got response status %d, wanted %d", status, tc.wantStatus)
			}
		})
	}
}

func TestNoProviderErrors(t *testing.T) {
	configs := []ContentConfig{
		{Type: Provider1},
		{Type: Provider2},
		{Type: Provider3},
		{Type: Provider2},
	}

	for name, tc := range map[string]struct {
		count             int
		offset            int
		expectItemSources []string
	}{
		"1 item": {
			count:             1,
			expectItemSources: []string{"1"},
		},
		"2 items": {
			count:             2,
			expectItemSources: []string{"1", "2"},
		},
		"3 items": {
			count:             3,
			expectItemSources: []string{"1", "2", "3"},
		},
		"3 items, offset 2": {
			count:             3,
			offset:            2,
			expectItemSources: []string{"3", "2", "1"},
		},
		"10 items": {
			count:             10,
			expectItemSources: []string{"1", "2", "3", "2", "1", "2", "3", "2", "1", "2"},
		},
		"10 items, offset 1": {
			count:             10,
			offset:            1,
			expectItemSources: []string{"2", "3", "2", "1", "2", "3", "2", "1", "2", "3"},
		},
		"10 items, offset 2": {
			count:             10,
			offset:            2,
			expectItemSources: []string{"3", "2", "1", "2", "3", "2", "1", "2", "3", "2"},
		},
		"10 items, offset 10": {
			count:             10,
			offset:            10,
			expectItemSources: []string{"3", "2", "1", "2", "3", "2", "1", "2", "3", "2"},
		},
		"10 items, offset 12": {
			count:             10,
			offset:            12,
			expectItemSources: []string{"1", "2", "3", "2", "1", "2", "3", "2", "1", "2"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			clients := map[Provider]Client{
				Provider1: &mockContentProvider{source: Provider1},
				Provider2: &mockContentProvider{source: Provider2},
				Provider3: &mockContentProvider{source: Provider3},
			}

			service, err := NewService(configs, clients, defaultTimeout)
			if err != nil {
				t.Fatalf("creating a service: %v", err)
			}

			req, _ := http.NewRequest(
				http.MethodGet,
				fmt.Sprintf("/?count=%d&offset=%d", tc.count, tc.offset),
				nil,
			)
			status, content := runRequest(t, service, req)

			if status != http.StatusOK {
				t.Fatalf("got response status %d", status)
			}
			if len(content) != len(tc.expectItemSources) {
				t.Fatalf("got %d items back, want %d", len(content), len(tc.expectItemSources))
			}

			for i, s := range tc.expectItemSources {
				if content[i].Source != s {
					t.Errorf("invalid source in item %d: %s, wanted: %s", i, content[i].Source, s)
				}
			}

			// Expect at least one call for each provider.
			for p, client := range clients {
				if calls := client.(*mockContentProvider).calls; calls > 1 {
					t.Errorf("unexpected number of calls to provider '%s': %d", p, calls)
				}
			}
		})
	}
}

func TestProviderErrors(t *testing.T) {
	Provider4 := Provider("4")
	Provider5 := Provider("5")

	configs := []ContentConfig{
		{Type: Provider1, Fallback: &Provider3}, // Fails, valid fallback. NOTE: Provider3 is only used as a fallback, so it should also get at most one call.
		{Type: Provider2, Fallback: nil},        // Succeeds.
		{Type: Provider4, Fallback: nil},        // Fails, no fallback.
		{Type: Provider5, Fallback: nil},        // Succeeds.
	}

	for name, tc := range map[string]struct {
		count             int
		offset            int
		expectItemSources []string
	}{
		"1 item": {
			count:             1,
			expectItemSources: []string{"3"},
		},
		"2 items": {
			count:             2,
			expectItemSources: []string{"3", "2"},
		},
		"3 items, 3rd provider fails": {
			count:             3,
			expectItemSources: []string{"3", "2"},
		},
		"10 items, 3rd provider fails": {
			count:             10,
			expectItemSources: []string{"3", "2"},
		},
		"1 item, offset 1": {
			count:             1,
			offset:            1,
			expectItemSources: []string{"2"},
		},
		"1 item, offset 2, 3rd provider fails": {
			count:             1,
			offset:            2,
			expectItemSources: []string{},
		},
		"1 item, offset 10, 3rd provider fails": {
			count:             1,
			offset:            10,
			expectItemSources: []string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			clients := map[Provider]Client{
				Provider1: &mockContentProvider{source: Provider1, shouldFail: true},
				Provider2: &mockContentProvider{source: Provider2, shouldFail: false},
				Provider3: &mockContentProvider{source: Provider3, shouldFail: false},
				Provider4: &mockContentProvider{source: Provider4, shouldFail: true},
				Provider5: &mockContentProvider{source: Provider5, shouldFail: false},
			}
			service, err := NewService(configs, clients, defaultTimeout)
			if err != nil {
				t.Fatalf("creating a service: %v", err)
			}

			req, _ := http.NewRequest(
				http.MethodGet,
				fmt.Sprintf("/?count=%d&offset=%d", tc.count, tc.offset),
				nil,
			)
			status, content := runRequest(t, service, req)

			if status != http.StatusOK {
				t.Fatalf("got response status %d", status)
			}
			if len(content) != len(tc.expectItemSources) {
				t.Fatalf("got %d items back, want %d", len(content), len(tc.expectItemSources))
			}

			for i, s := range tc.expectItemSources {
				if content[i].Source != s {
					t.Errorf("invalid source in item %d: %s, wanted: %s", i, content[i].Source, s)
				}
			}

			// Expect at most one call for each provider.
			for p, client := range clients {
				if calls := client.(*mockContentProvider).calls; calls > 1 {
					t.Errorf("unexpected number of calls to provider '%s': %d", p, calls)
				}
			}
		})
	}
}

func TestResponseTimeout(t *testing.T) {
	clients := map[Provider]Client{
		Provider1: &mockContentProvider{source: Provider1, responseDelay: 1 * time.Second},
		Provider2: &mockContentProvider{source: Provider2, responseDelay: 2 * time.Second},
		Provider3: &mockContentProvider{source: Provider3, responseDelay: 3 * time.Second},
	}
	configs := []ContentConfig{
		{Type: Provider1},
		{Type: Provider2},
		{Type: Provider3},
	}
	timeout := time.Second
	service, err := NewService(configs, clients, timeout)
	if err != nil {
		t.Fatalf("creating a service: %v", err)
	}

	done := make(chan struct{})
	var status int
	go func() {
		defer close(done)
		req, _ := http.NewRequest(http.MethodGet, "/?count=3", nil)
		status, _ = runRequest(t, service, req)
	}()

	select {
	case <-done:
		if status != http.StatusInternalServerError {
			t.Fatalf("got unexpected response status %d", status)
		}
	case <-time.After(3 * timeout):
		t.Fatal("request still processing")
	}
}
