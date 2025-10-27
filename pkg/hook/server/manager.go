package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/gk7790/gk-zap/pkg/utils/log"
	"github.com/gk7790/gk-zap/pkg/utils/xlog"

	"github.com/gk7790/gk-zap/pkg/utils/util"
)

type Manager struct {
	loginPlugins       []Plugin
	newProxyPlugins    []Plugin
	closeProxyPlugins  []Plugin
	pingPlugins        []Plugin
	newWorkConnPlugins []Plugin
	newUserConnPlugins []Plugin
}

func NewManager() *Manager {
	return &Manager{
		loginPlugins:       make([]Plugin, 0),
		newProxyPlugins:    make([]Plugin, 0),
		closeProxyPlugins:  make([]Plugin, 0),
		pingPlugins:        make([]Plugin, 0),
		newWorkConnPlugins: make([]Plugin, 0),
		newUserConnPlugins: make([]Plugin, 0),
	}
}

func (m *Manager) Register(p Plugin) {
	if p.IsSupport(OpLogin) {
		m.loginPlugins = append(m.loginPlugins, p)
	}
	if p.IsSupport(OpNewProxy) {
		m.newProxyPlugins = append(m.newProxyPlugins, p)
	}
	if p.IsSupport(OpCloseProxy) {
		m.closeProxyPlugins = append(m.closeProxyPlugins, p)
	}
	if p.IsSupport(OpPing) {
		m.pingPlugins = append(m.pingPlugins, p)
	}
	if p.IsSupport(OpNewWorkConn) {
		m.newWorkConnPlugins = append(m.newWorkConnPlugins, p)
	}
	if p.IsSupport(OpNewUserConn) {
		m.newUserConnPlugins = append(m.newUserConnPlugins, p)
	}
}

func (m *Manager) Login(content *LoginContent) (*LoginContent, error) {
	if len(m.loginPlugins) == 0 {
		return content, nil
	}
	var (
		res = &Response{
			Reject:   false,
			Unchange: true,
		}
		retContent any
		err        error
	)

	reqid, _ := util.RandID()
	ctx := context.Background()
	log.Infof("reqid: " + reqid)
	for _, p := range m.loginPlugins {
		res, retContent, err = p.Handle(ctx, OpLogin, *content)
		if err != nil {
			log.Warnf("send Login request to plugin [%s] error: %v", p.Name(), err)
			return nil, errors.New("send Login request to plugin error")
		}
		if res.Reject {
			return nil, fmt.Errorf("%s", res.RejectReason)
		}
		if !res.Unchange {
			content = retContent.(*LoginContent)
		}
	}
	return content, nil
}

func (m *Manager) NewProxy(content *NewProxyContent) (*NewProxyContent, error) {
	if len(m.newProxyPlugins) == 0 {
		return content, nil
	}
	var (
		res = &Response{
			Reject:   false,
			Unchange: true,
		}
		retContent any
		err        error
	)
	reqid, _ := util.RandID()
	xl := xlog.New().AppendPrefix("reqid: " + reqid)
	ctx := xlog.NewContext(context.Background(), xl)
	ctx = NewReqidContext(ctx, reqid)
	for _, p := range m.newProxyPlugins {
		res, retContent, err = p.Handle(ctx, OpNewProxy, *content)
		if err != nil {
			xl.Warnf("send NewProxy request to plugin [%s] error: %v", p.Name(), err)
			return nil, errors.New("send NewProxy request to plugin error")
		}
		if res.Reject {
			return nil, fmt.Errorf("%s", res.RejectReason)
		}
		if !res.Unchange {
			content = retContent.(*NewProxyContent)
		}
	}
	return content, nil
}

func (m *Manager) NewWorkConn(content *NewWorkConnContent) (*NewWorkConnContent, error) {

	return content, nil
}

func (m *Manager) Ping(content *PingContent) (*PingContent, error) {
	if len(m.pingPlugins) == 0 {
		return content, nil
	}
	var (
		res = &Response{
			Reject:   false,
			Unchange: true,
		}
		retContent any
		err        error
	)
	reqid, _ := util.RandID()
	xl := xlog.New().AppendPrefix("reqid: " + reqid)
	ctx := xlog.NewContext(context.Background(), xl)
	ctx = NewReqidContext(ctx, reqid)
	for _, p := range m.pingPlugins {
		res, retContent, err = p.Handle(ctx, OpPing, *content)
		if err != nil {
			xl.Warnf("send Ping request to plugin [%s] error: %v", p.Name(), err)
			return nil, errors.New("send Ping request to plugin error")
		}
		if res.Reject {
			return nil, fmt.Errorf("%s", res.RejectReason)
		}
		if !res.Unchange {
			content = retContent.(*PingContent)
		}
	}
	return content, nil
}
