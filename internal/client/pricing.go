package client

import (
	"context"
	"fmt"
)

// Pricing provides methods for accessing pricing and metadata
type Pricing struct {
	client *Client
}

// NewPricing creates a new pricing service
func NewPricing(client *Client) *Pricing {
	return &Pricing{client: client}
}

// Get retrieves pricing information including locations, plans, and templates
func (p *Pricing) Get(ctx context.Context) (*PricingResponse, error) {
	var result PricingResponse
	err := p.client.Get(ctx, "/pricing", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing: %w", err)
	}
	return &result, nil
}
