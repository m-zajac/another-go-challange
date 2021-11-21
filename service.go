package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	defaultTimeout = time.Second * 5
)

// Service is the main application service object.
type Service struct {
	clients        map[Provider]Client
	contentConfigs []ContentConfig
	timeout        time.Duration
}

// NewDefaultService returns a service with default configuration.
func NewDefaultService() (*Service, error) {
	return NewService(
		DefaultConfig,
		map[Provider]Client{
			Provider1: SampleContentProvider{Source: Provider1},
			Provider2: SampleContentProvider{Source: Provider2},
			Provider3: SampleContentProvider{Source: Provider3},
		},
		defaultTimeout,
	)
}

// NewService returns a service configured with the given configs and clients.
func NewService(configs []ContentConfig, clients map[Provider]Client, timeout time.Duration) (*Service, error) {
	for _, cfg := range configs {
		if _, ok := clients[cfg.Type]; !ok {
			return nil, fmt.Errorf("no client provided for provider '%s'", cfg.Type)
		}
	}

	return &Service{
		clients:        clients,
		contentConfigs: configs,
		timeout:        timeout,
	}, nil
}

// GetContent returns `count` number of content items, fetched from the configured providers.
func (s *Service) GetContent(ctx context.Context, userIP string, count int, offset int) ([]*ContentItem, error) {
	if count <= 0 || offset < 0 {
		return nil, fmt.Errorf("invalid count or offset parameters")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	responses, err := s.getConfigResponses(ctx, userIP, count, offset)
	if err != nil {
		return nil, err
	}

	var items []*ContentItem
	for _, v := range responses {
		if v.err != nil {
			// There was an error for this item, so return with what we collected so far.
			break
		}
		items = append(items, v.item)
	}

	if offset >= len(items) {
		return nil, nil
	}
	return items[offset:], nil
}

// configResponse is a helper type for storing the result of fetching data for given config element.
type configResponse struct {
	item *ContentItem
	err  error
}

func (s *Service) getConfigResponses(ctx context.Context, userIP string, count int, offset int) ([]*configResponse, error) {
	requestConfigs := s.prepareConfigsForRequest(count, offset)

	// Check how many items do we need from each provider.
	providerCounts := make(map[Provider]int)
	for _, cfg := range requestConfigs {
		providerCounts[cfg.Type]++
	}

	// Collect response promises from each provider.
	responsePromises := make(map[Provider]<-chan *configResponse)
	for provider, count := range providerCounts {
		responsePromises[provider] = s.getPromiseForProvider(ctx, provider, userIP, count)
	}

	// First pass: fetch data from providers without any fallbacks.
	responses := make([]*configResponse, len(requestConfigs))
	for i, cfg := range requestConfigs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case v, ok := <-responsePromises[cfg.Type]:
			if !ok {
				responses[i] = &configResponse{err: errors.New("not enough items")}
				continue
			}
			responses[i] = v
		}
	}

	// Second pass: for each error in the responses try to use a fallback provider.
	fallbackProviderCounts := make(map[Provider]int)
	for i, cfg := range requestConfigs {
		if responses[i].err == nil {
			continue
		}
		if cfg.Fallback == nil {
			// Error and no fallback - we won't return response for this and any of the next items, so we can stop here.
			break
		}
		fallbackProviderCounts[*cfg.Fallback]++
	}
	if len(fallbackProviderCounts) != 0 {
		// Collect response promises for fallbacks.
		responsePromises = make(map[Provider]<-chan *configResponse)
		for provider, count := range fallbackProviderCounts {
			responsePromises[provider] = s.getPromiseForProvider(ctx, provider, userIP, count)
		}

		// Fill the requestConfigs with fallback responses.
		for i, cfg := range requestConfigs {
			if responses[i].err == nil {
				continue
			}
			if cfg.Fallback == nil {
				break
			}
			provider := *cfg.Fallback
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case v, ok := <-responsePromises[provider]:
				if !ok {
					responses[i] = &configResponse{err: errors.New("not enough items")}
					continue
				}
				responses[i] = v
			}
		}
	}

	return responses, nil
}

// prepareConfigsForRequest returns a list of configs that configure each item that is used for generating response.
// It takes configured "configs" and repeats them to make a slice of len `count+offset`.
func (s *Service) prepareConfigsForRequest(count int, offset int) []ContentConfig {
	var configs []ContentConfig
	for i := 0; i < count+offset; i++ {
		idx := i % len(s.contentConfigs)
		cfg := s.contentConfigs[idx]
		configs = append(configs, cfg)
	}

	return configs
}

// getResponseForConfig returns a "promise" with response data for given config and count.
func (s *Service) getPromiseForProvider(ctx context.Context, p Provider, userIP string, count int) <-chan *configResponse {
	client, ok := s.clients[p]
	if !ok {
		panic(fmt.Sprintf("no client configured for provider %s", p))
	}

	out := make(chan *configResponse, count)
	go func() {
		defer close(out)

		items, err := client.GetContent(userIP, count)
		if err != nil {
			log.Printf("fetch data failed (provider:'%s' count:%d)", p, count)
			out <- &configResponse{err: err}
			return
		}

		log.Printf("fetched data (provider:'%s' count:%d)", p, count)

		// We want to be sure that we don't have more items than the channel buffer size.
		// Otherwise this goroutine won't be able to finish.
		items = items[:count]

		for _, item := range items {
			out <- &configResponse{item: item}
		}
	}()

	return out
}
