package discovery

import (
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming/discovery"
	"github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster/resolver"
)

func init() {
	resolver.Register(discovery.Builder())
}
