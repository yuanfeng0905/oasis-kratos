package http

import (
	"context"
	"time"

	"github.com/yuanfeng0905/oasis-kratos/pkg/conf/env"
	"github.com/yuanfeng0905/oasis-kratos/pkg/log"
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming"
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming/discovery"
	bm "github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster"
)

type ServerConfig struct {
	bm.ServerConfig
}

// 支持服务自动注册
type Server struct {
	Engine *bm.Engine
	// discovery cancel
	discoveryCancel context.CancelFunc
}

func DefaultServer(conf *ServerConfig) *Server {
	e := bm.DefaultServer(&bm.ServerConfig{
		Network:      conf.Network,
		Addr:         conf.Addr,
		Timeout:      conf.Timeout,
		ReadTimeout:  conf.ReadTimeout,
		WriteTimeout: conf.WriteTimeout,
	})

	return &Server{Engine: e}
}

func NewServer(conf *ServerConfig) *Server {
	e := bm.NewServer(&bm.ServerConfig{
		Network:      conf.Network,
		Addr:         conf.Addr,
		Timeout:      conf.Timeout,
		ReadTimeout:  conf.ReadTimeout,
		WriteTimeout: conf.WriteTimeout,
	})

	return &Server{Engine: e}
}

func (s *Server) Start() (err error) {
	if err = s.Engine.Start(); err != nil {
		return
	}

	for {
		if s.Engine.Server() == nil {
			time.Sleep(100 * time.Millisecond)
		}
		break
	}
	return s.registerSelf()
}

func (s *Server) registerSelf() (err error) {
	if env.DiscoveryNodes == "" {
		log.Info(`blademaster: discovery not be enabled. params "-discovery.nodes" or env(DISCOVERY_NODES) not set.`)
		return nil
	}

	dis := discovery.New(nil)
	inst := &naming.Instance{
		Zone:     env.Zone,
		Env:      env.DeployEnv,
		AppID:    env.AppID,
		Hostname: env.Hostname,
		Addrs: []string{
			"http://" + s.Engine.Server().Addr, // default scheme only support HTTP
		},
	}
	s.discoveryCancel, err = dis.Register(context.Background(), inst)
	return
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.discoveryCancel()
	return s.Engine.Shutdown(ctx)
}
