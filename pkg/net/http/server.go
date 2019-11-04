package http

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/yuanfeng0905/oasis-kratos/pkg/conf/env"
	"github.com/yuanfeng0905/oasis-kratos/pkg/log"
	bm "github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster"

	"github.com/yuanfeng0905/oasis-kratos/pkg/naming"
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming/discovery"
)

type ServerConfig struct {
	bm.ServerConfig
}

type Server struct {
	engine *bm.Engine
	// discovery cannelFunc
	discoveryCancel context.CancelFunc
}

// DefaultServer returns an Engine instance with the Recovery and Logger middleware already attached.
func DefaultServer(conf *ServerConfig) *Server {
	engine := bm.NewServer(&bm.ServerConfig{
		Network:      conf.Network,
		Addr:         conf.Addr,
		Timeout:      conf.Timeout,
		ReadTimeout:  conf.ReadTimeout,
		WriteTimeout: conf.WriteTimeout,
	})
	engine.Use(bm.Recovery(), bm.Trace(), bm.Logger())
	return &Server{engine: engine}
}

func NewServer(conf *bm.ServerConfig) *Server {
	return &Server{engine: bm.NewServer(conf)}
}

func (s *Server) registerSelf() (err error) {
	if env.DiscoveryNodes == "" {
		log.Info(`blademaster: discovery not be enabled. params "-discovery.nodes" or env(DISCOVERY_NODES) not set.`)
		return nil
	}

	for {
		if s.engine.Server() == nil {
			time.Sleep(100 * time.Millisecond)
		}
		break
	}

	dis := discovery.New(nil)
	inst := &naming.Instance{
		Zone:     env.Zone,
		Env:      env.DeployEnv,
		AppID:    env.AppID,
		Hostname: env.Hostname,
		Addrs: []string{
			"http://" + s.engine.Server().Addr, // default scheme only support HTTP
		},
	}
	s.discoveryCancel, err = dis.Register(context.Background(), inst)

	return
}

func (s *Server) Start() error {
	if err := s.engine.Start(); err != nil {
		return err
	}

	// register discovery
	if err := s.registerSelf(); err != nil {
		panic(errors.Wrapf(err, "blademaster: engine.registerSelf error: %v", err))
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) {
	// unregister discovery
	s.discoveryCancel()
	s.engine.Shutdown(ctx)
}
