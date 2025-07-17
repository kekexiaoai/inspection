package prom

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Client represents a Prometheus query client.
type Client struct {
	api    v1.API
	ctx    context.Context
	cancel context.CancelFunc
}

// Option configures the Client.
type Option func(*Client)

// WithTimeout sets the context timeout for queries.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.ctx, c.cancel = context.WithTimeout(context.Background(), d)
	}
}

// NewClient creates a new Prometheus query client.
func NewClient(addr string, opts ...Option) (*Client, error) {
	client, err := api.NewClient(api.Config{
		Address: addr,
	})
	if err != nil {
		return nil, err
	}

	c := &Client{
		api:    v1.NewAPI(client),
		ctx:    context.Background(),
		cancel: func() {},
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Query performs an instant query and returns the result.
func (c *Client) Query(query string, ts time.Time) (model.Value, v1.Warnings, error) {
	return c.api.Query(c.ctx, query, ts)
}

// QueryRange performs a range query and returns the result.
func (c *Client) QueryRange(query string, r v1.Range) (model.Value, v1.Warnings, error) {
	return c.api.QueryRange(c.ctx, query, r)
}

// Alerts retrieves the current alert overview.
func (c *Client) Alerts() (v1.AlertsResult, error) {
	return c.api.Alerts(c.ctx)
}

// AlertManagers retrieves the list of alert managers.
func (c *Client) AlertManagers() (v1.AlertManagersResult, error) {
	return c.api.AlertManagers(c.ctx)
}

// CleanTombstones cleans up tombstones from the TSDB.
func (c *Client) CleanTombstones() error {
	return c.api.CleanTombstones(c.ctx)
}

// Close cancels the context to release resources.
func (c *Client) Close() {
	c.cancel()
}

// Range defines a time range for range queries.
type Range struct {
	Start time.Time
	End   time.Time
	Step  time.Duration
}

// NewRange creates a new Range for QueryRange.
func NewRange(start, end time.Time, step time.Duration) v1.Range {
	return v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}
}
