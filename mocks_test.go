package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

type mockContentProvider struct {
	source        Provider
	shouldFail    bool
	responseDelay time.Duration
	calls         int
	m             sync.Mutex
}

// GetContent returns content items given a user IP, and the number of content items desired.
func (cp *mockContentProvider) GetContent(userIP string, count int) ([]*ContentItem, error) {
	if cp.responseDelay > 0 {
		time.Sleep(cp.responseDelay)
	}

	cp.m.Lock()
	defer cp.m.Unlock()

	cp.calls++

	if cp.shouldFail {
		return nil, fmt.Errorf("test error")
	}

	resp := make([]*ContentItem, count)
	for i := range resp {
		resp[i] = &ContentItem{
			ID:     strconv.Itoa(rand.Int()),
			Title:  "test title",
			Source: string(cp.source),
			Expiry: time.Now(),
		}
	}

	return resp, nil
}
