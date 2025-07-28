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
	api          v1.API
	ctx          context.Context
	cancel       context.CancelFunc
	queryTimeout time.Duration // 查询超时时间
}

// Option configures the Client.
type Option func(*Client)

// WithTimeout sets the default timeout for queries.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.queryTimeout = timeout
	}
}

// WithContext sets a base context for the client (e.g., for authentication).
func WithContext(ctx context.Context) Option {
	return func(c *Client) {
		c.ctx, c.cancel = context.WithCancel(ctx)
	}
}

const defaultTimeout = 30 * time.Second

// NewClient creates a new Prometheus query client.
func NewClient(addr string, opts ...Option) (*Client, error) {
	client, err := api.NewClient(api.Config{
		Address: addr,
	})
	if err != nil {
		return nil, err
	}

	c := &Client{
		api:          v1.NewAPI(client),
		ctx:          context.Background(),
		cancel:       func() {},      // 默认空函数，避免nil调用
		queryTimeout: defaultTimeout, // 默认30秒超时
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// WithTimeout creates a new Client instance with a specific timeout,
// inheriting all other configuration from the original client.
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	return &Client{
		api:          c.api,
		ctx:          c.ctx,
		cancel:       c.cancel,
		queryTimeout: timeout,
	}
}

// WithContext creates a new Client instance with a specific context,
// inheriting all other configuration from the original client.
func (c *Client) WithContext(ctx context.Context) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{
		api:          c.api,
		ctx:          ctx,
		cancel:       cancel,
		queryTimeout: c.queryTimeout,
	}
}

// Query performs an instant query using the configured timeout and returns the result.
func (c *Client) Query(query string, ts time.Time) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.queryTimeout)
	defer cancel()
	return c.api.Query(ctx, query, ts)
}

// QueryRange performs a range query using the configured timeout and returns the result.
func (c *Client) QueryRange(query string, r v1.Range) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.queryTimeout)
	defer cancel()
	return c.api.QueryRange(ctx, query, r)
}

// Targets retrieves the current overview using the configured timeout.
func (c *Client) Targets() (v1.TargetsResult, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.queryTimeout)
	defer cancel()
	return c.api.Targets(ctx)
}

// Alerts retrieves the current alert overview using the configured timeout.
func (c *Client) Alerts() (v1.AlertsResult, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.queryTimeout)
	defer cancel()
	return c.api.Alerts(ctx)
}

// AlertManagers retrieves the list of alert managers using the configured timeout.
func (c *Client) AlertManagers() (v1.AlertManagersResult, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.queryTimeout)
	defer cancel()
	return c.api.AlertManagers(ctx)
}

// CleanTombstones cleans up tombstones from the TSDB using the configured timeout.
func (c *Client) CleanTombstones() error {
	ctx, cancel := context.WithTimeout(c.ctx, c.queryTimeout)
	defer cancel()
	return c.api.CleanTombstones(ctx)
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
