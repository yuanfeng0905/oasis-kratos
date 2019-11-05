package discovery

import (
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming/discovery"
	"github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster/resolver"
)

// 默认注册 discovery 的 resolver 实现
func init() {
	resolver.Register(discovery.Builder())
}
