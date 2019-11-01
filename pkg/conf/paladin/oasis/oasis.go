package oasis

import (
	"context"
	"errors"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/yuanfeng0905/oasis-kratos/pkg/conf/env"
	"github.com/yuanfeng0905/oasis-kratos/pkg/conf/paladin"
	"github.com/yuanfeng0905/oasis-kratos/pkg/ecode"
	http "github.com/yuanfeng0905/oasis-kratos/pkg/net/http/blademaster"
	xtime "github.com/yuanfeng0905/oasis-kratos/pkg/time"
)

var (
	_            paladin.Client = &oasis{}
	defaultValue                = ""
)

type Diff struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
}

type oasisWatcher struct {
	keys []string
	C    chan paladin.Event
}

func newOasisWatcher(keys []string) *oasisWatcher {
	return &oasisWatcher{keys: keys, C: make(chan paladin.Event, 5)}
}

func (ow *oasisWatcher) HasKey(key string) bool {
	if len(ow.keys) == 0 {
		return true
	}
	for _, k := range ow.keys {
		if k == key {
			return true
		}
	}
	return false
}

func (ow *oasisWatcher) Handle(event paladin.Event) {
	select {
	case ow.C <- event:
	default:
		log.Printf("paladin: event channel full discard ns %s update event", event.Key)
	}
}

type oasis struct {
	client    *http.Client
	values    *paladin.Map
	wmu       sync.RWMutex
	watchers  map[*oasisWatcher]struct{}
	nLock     sync.RWMutex
	namesRepo map[string]*Diff
}

type Config struct {
	AppID    string `json:"app_id"`
	Env      string `json:"env"`
	Zone     string `json:"zone"`
	CacheDir string `json:"cache_dir`
	//Names    []string `json:"names"` // 监听的配置文件名
}

type oasisDriver struct{}

var (
	confAppID, confCluster, confCacheDir, confMetaAddr, confNamespaces string
)

func init() {
	paladin.Register(PaladinDriverOasis, &oasisDriver{})
}

func buildConfigForOasis() (c *Config, err error) {
	c = &Config{
		AppID:    env.AppID,
		Env:      env.DeployEnv,
		Zone:     env.Zone,
		CacheDir: confCacheDir,
	}
	return
}

// New new an oasis config client.
func (ad *oasisDriver) New() (paladin.Client, error) {
	c, err := buildConfigForOasis()
	if err != nil {
		return nil, err
	}
	return ad.new(c)
}

func (ad *oasisDriver) new(conf *Config) (paladin.Client, error) {
	if conf == nil {
		err := errors.New("invalid oasis conf")
		return nil, err
	}
	a := &oasis{
		client: http.NewClient(&http.ClientConfig{
			Dial:      xtime.Duration(3 * time.Second),
			Timeout:   xtime.Duration(40 * time.Second),
			KeepAlive: xtime.Duration(40 * time.Second),
		}),
		values:    new(paladin.Map),
		watchers:  make(map[*oasisWatcher]struct{}),
		namesRepo: make(map[string]*Diff),
	}

	go a.watchproc()

	return a, nil
}

// loadValues
func (a *oasis) loadValues(keys []string) (values map[string]*paladin.Value, err error) {
	values = make(map[string]*paladin.Value, len(keys))
	for _, k := range keys {
		if values[k], err = a.loadValue(k); err != nil {
			return
		}
	}
	return
}

// loadValue
func (a *oasis) loadValue(key string) (*paladin.Value, error) {
	params := url.Values{}
	params.Set("app_id", env.AppID)
	params.Set("name", key)
	params.Set("env", env.DeployEnv)
	params.Set("zone", env.Zone)
	params.Set("ip", "")

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Version int    `json:"version"`
			Content string `json:"content"`
			MD5     string `json:"md5"`
		} `json:"data"`
	}
	if err := a.client.Get(context.Background(),
		"discovery://infra.config/api/config/fetch", "", params, &resp); err != nil {
		return nil, err
	}

	a.nLock.Lock()
	if _, ok := a.namesRepo[key]; ok {
		a.namesRepo[key].Version = resp.Data.Version // update version for listener
	}
	a.nLock.Unlock()

	// update local memory
	value := paladin.NewValue(resp.Data.Content, resp.Data.Content)
	raws := a.values.Load()
	raws[key] = value
	a.values.Store(raws)

	return value, nil
}

// reloadValue reload value by key and send event
func (a *oasis) reloadValue(key string) (err error) {
	var (
		value    *paladin.Value
		rawValue string
	)
	value, err = a.loadValue(key)
	if err != nil {
		return
	}
	rawValue, err = value.Raw()
	if err != nil {
		return
	}

	a.wmu.RLock()
	n := 0
	for w := range a.watchers {
		if w.HasKey(key) {
			n++
			// FIXME(Colstuwjx): check change event and send detail type like EventAdd\Update\Delete.
			w.Handle(paladin.Event{Event: paladin.EventUpdate, Key: key, Value: rawValue})
		}
	}
	a.wmu.RUnlock()
	log.Printf("paladin: reload config: %s events: %d\n", key, n)
	return
}

func (a *oasis) watchUpdate() ([]*Diff, error) {
	var params struct {
		AppID string  `json:"app_id"`
		Env   string  `json:"env"`
		Zone  string  `json:"zone"`
		IP    string  `json:"ip"`
		Items []*Diff `json:"items"` //关注的配置项
	}

	a.nLock.RLock()
	for _, item := range a.namesRepo {
		params.Items = append(params.Items, item)
	}
	a.nLock.RUnlock()

	req, err := a.client.NewJSONRequest("POST", "discovery://infra.config/api/config/listeners", params)
	if err != nil {
		log.Printf("paladin: create request error: %s", err)
		return nil, err
	}

	var resp struct {
		Code    int     `json:"code"`
		Message string  `json:"message"`
		Data    []*Diff `json:"data"`
	}
	if err := a.client.JSON(context.Background(), req, &resp); err != nil {
		log.Printf("paladin: create listener error: %s", err)
		return nil, err
	}

	// not modify
	if resp.Code == ecode.NotModified.Code() {
		return nil, nil
	}

	// update
	if resp.Code == 0 {
		return resp.Data, nil
	}

	return nil, errors.New(resp.Message)
}

// oasis config daemon to watch remote oasis notifications
func (a *oasis) watchproc() {
	for {
		if len(a.namesRepo) == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		diffs, err := a.watchUpdate()
		if err != nil {
			log.Printf("paladin: watchUpdate error: %s", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if diffs != nil {
			for _, diff := range diffs {
				a.reloadValue(diff.Name)
			}
		}
	}
}

// Get return value by key.
func (a *oasis) Get(key string) *paladin.Value {
	// 第一次加载，尝试从远程获取
	// TODO 这里并发会出现多次请求，待优化
	if _, err := a.values.Get(key).Raw(); err == paladin.ErrNotExist {
		val, err := a.loadValue(key)
		if err != nil {
			log.Printf("pladin: loadValue error: %s", err)
			return val
		}
		// 本地主动订阅
		a.subscribeNames(key)
	}

	return a.values.Get(key)
}

// GetAll return value map.
func (a *oasis) GetAll() *paladin.Map {
	return a.values
}

func (a *oasis) subscribeNames(keys ...string) {
	a.nLock.Lock()
	for _, key := range keys {
		if _, ok := a.namesRepo[key]; !ok {
			a.namesRepo[key] = &Diff{Name: key, Version: -1} // default version = -1
		}
	}
	a.nLock.Unlock()
}

// WatchEvent watch with the specified keys.
func (a *oasis) WatchEvent(ctx context.Context, keys ...string) <-chan paladin.Event {
	aw := newOasisWatcher(keys)
	a.subscribeNames(keys...)

	a.wmu.Lock()
	a.watchers[aw] = struct{}{}
	a.wmu.Unlock()
	return aw.C
}

// Close close watcher.
func (a *oasis) Close() (err error) {
	a.wmu.RLock()
	for w := range a.watchers {
		close(w.C)
	}
	a.wmu.RUnlock()
	return
}
