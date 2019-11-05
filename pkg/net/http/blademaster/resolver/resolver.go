package resolver

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/yuanfeng0905/oasis-kratos/pkg/conf/env"
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming"

	// default import
	_ "github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster/resolver/discovery"
)

var (
	m = make(map[string]naming.Builder)
)

// ResolverTransport wraps a RoundTripper.
type ResolverTransport struct {

	// The actual RoundTripper to use for the request. A nil
	// RoundTripper defaults to http.DefaultTransport.
	http.RoundTripper
}

// Register resolver
func Register(b naming.Builder) {
	m[b.Scheme()] = b
}

// NewResolverTransport NewResolverTransport
func NewResolverTransport(rt http.RoundTripper) *ResolverTransport {
	return &ResolverTransport{RoundTripper: rt}
}

func (t *ResolverTransport) filter(instances []*naming.Instance) (*url.URL, error) {
	urls := []*url.URL{}
	for _, _inst := range instances {
		for _, addr := range _inst.Addrs {
			if _url, err := url.Parse(addr); err == nil && (_url.Scheme == "http" || _url.Scheme == "https") {
				urls = append(urls, _url)
			}
		}
	}

	if len(urls) == 0 {
		return nil, errors.New("invalid target address")
	}

	return urls[0], nil
}

func (t *ResolverTransport) pickInstances(appid string, builder naming.Builder) (instances []*naming.Instance, err error) {
	resolver := builder.Build(appid)

	ev := resolver.Watch()
	_, ok := <-ev
	if !ok {
		err = errors.New("discovery watch failed")
		return
	}

	info, ok := resolver.Fetch(context.Background())
	if !ok {
		err = errors.New("poll node fail")
		return
	}

	instances = []*naming.Instance{}
	if _insts, ok := info.Instances[env.Zone]; ok {
		instances = _insts
		return
	}

	// all zone?
	for _, _insts := range info.Instances {
		instances = append(instances, _insts...)
	}

	return
}

// RoundTrip implements the RoundTripper interface
func (t *ResolverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}

	if len(m) == 0 {
		return rt.RoundTrip(req)
	}

	if b, ok := m[req.URL.Scheme]; ok {
		// url format: discovery://appid/xxxx
		newReq := new(http.Request)
		*newReq = *req

		insts, err := t.pickInstances(req.URL.Hostname(), b)
		if err != nil {
			return nil, err
		}

		_url, err := t.filter(insts)
		if err != nil {
			return nil, err
		}

		newReq.Host = _url.Host
		newReq.URL.Scheme = _url.Scheme
		newReq.URL.Host = _url.Host

		resp, err := rt.RoundTrip(newReq)
		if err != nil && resp != nil {
			resp.Request.Host = req.Host
			resp.Request.URL.Scheme = req.URL.Scheme
			resp.Request.URL.Host = req.URL.Host
		}

		return resp, err
	}

	resp, err := rt.RoundTrip(req)
	return resp, err
}
