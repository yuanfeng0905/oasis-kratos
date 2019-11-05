package http

import (
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming/discovery"
	bm "github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster"
	"github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster/resolver"
)

func init() {
	// default register discovery
	resolver.Register(discovery.Builder())
}

type ClientConfig struct {
	bm.ClientConfig
}

func NewClient(c *ClientConfig) *bm.Client {

	return bm.NewClient(&bm.ClientConfig{
		Dial:      c.Dial,
		Timeout:   c.Timeout,
		KeepAlive: c.KeepAlive,
		Breaker:   c.Breaker,
		URL:       c.URL,
		Host:      c.Host,
	})
}
