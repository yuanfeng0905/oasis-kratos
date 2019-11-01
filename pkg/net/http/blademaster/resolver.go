package blademaster

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/yuanfeng0905/oasis-kratos/pkg/conf/env"
	"github.com/yuanfeng0905/oasis-kratos/pkg/naming"
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

// Register
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	select {
	case <-ev:
	case <-ctx.Done():
		err = errors.New("fetch node timeout")
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

	// url format: discovery://appid/xxxx
	oldURL := new(url.URL)
	*oldURL = *req.URL

	if b, ok := m[req.URL.Scheme]; ok {
		insts, err := t.pickInstances(req.URL.Hostname(), b)
		if err != nil {
			return nil, err
		}

		_url, err := t.filter(insts)
		if err != nil {
			return nil, err
		}

		req.URL.Scheme = _url.Scheme
		req.URL.Host = _url.Host
	}

	resp, err := rt.RoundTrip(req)
	resp.Request.URL = oldURL // restore

	return resp, err
}
