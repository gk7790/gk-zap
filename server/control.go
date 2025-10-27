package server

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	hook "github.com/gk7790/gk-zap/pkg/hook/server"
	"github.com/gk7790/gk-zap/pkg/utils/log"
	"github.com/gk7790/gk-zap/pkg/utils/util"
	"github.com/gk7790/gk-zap/pkg/utils/wait"
	"github.com/gk7790/gk-zap/pkg/utils/xlog"
	"github.com/gk7790/gk-zap/server/metrics"
	"github.com/samber/lo"

	"github.com/gk7790/gk-zap/pkg/auth"
	m "github.com/gk7790/gk-zap/pkg/config/model"
	"github.com/gk7790/gk-zap/pkg/msg"
	"github.com/gk7790/gk-zap/pkg/utils/version"
)

type ControlManager struct {
	ctlsByRunID map[string]*Control
	mu          sync.RWMutex
}

func NewControlManager() *ControlManager {
	return &ControlManager{
		ctlsByRunID: make(map[string]*Control),
	}
}

func (cm *ControlManager) Add(runID string, ctl *Control) (old *Control) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var ok bool
	old, ok = cm.ctlsByRunID[runID]
	if ok {
		old.Replaced(ctl)
	}
	cm.ctlsByRunID[runID] = ctl
	return
}

func (cm *ControlManager) Del(runID string, ctl *Control) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if c, ok := cm.ctlsByRunID[runID]; ok && c == ctl {
		delete(cm.ctlsByRunID, runID)
	}
}

func (cm *ControlManager) GetByID(runID string) (ctl *Control, ok bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ctl, ok = cm.ctlsByRunID[runID]
	return
}

func (cm *ControlManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, ctl := range cm.ctlsByRunID {
		ctl.Close()
	}
	cm.ctlsByRunID = make(map[string]*Control)
	return nil
}

type Control struct {
	runID string

	//
	mu sync.RWMutex

	// 身份验证器
	authVerifier auth.Verifier

	// 消息调度器
	msgDispatcher *msg.Dispatcher

	// hook 管理
	hookManager *hook.Manager

	// 登入信息
	loginMsg *msg.Login

	// 控制器连接 connection
	conn net.Conn

	// 上下文
	ctx context.Context
	// 本链接的日志
	xl *xlog.Logger
	// Server configuration information
	serverCfg *m.ServerConfig

	// 工作连接 work connections
	workConnCh chan net.Conn

	// 上次收到Ping消息
	lastPing atomic.Value

	// pool count
	poolCount int

	doneCh chan struct{}
}

func NewControl(ctx context.Context, ctlConn net.Conn, hookManager *hook.Manager, loginMsg *msg.Login, serverCfg *m.ServerConfig) (*Control, error) {
	poolCount := loginMsg.PoolCount
	if poolCount > int(serverCfg.Transport.MaxPoolCount) {
		poolCount = int(serverCfg.Transport.MaxPoolCount)
	}
	ctl := &Control{
		runID:       loginMsg.RunID,
		ctx:         ctx,
		conn:        ctlConn,
		hookManager: hookManager,
		serverCfg:   serverCfg,
		xl:          xlog.FromContextSafe(ctx),
		doneCh:      make(chan struct{}),
	}
	ctl.lastPing.Store(time.Now())
	ctl.msgDispatcher = msg.NewDispatcher(ctl.conn)
	return ctl, nil
}

func (ctl *Control) registerMsgHandlers() {
	ctl.msgDispatcher.RegisterHandler(&msg.NewProxy{}, ctl.handleNewProxy)
	ctl.msgDispatcher.RegisterHandler(&msg.Ping{}, ctl.handlePing)
	//ctl.msgDispatcher.RegisterHandler(&msg.NatHoleVisitor{}, msg.AsyncHandler(ctl.handleNatHoleVisitor))
	//ctl.msgDispatcher.RegisterHandler(&msg.NatHoleClient{}, msg.AsyncHandler(ctl.handleNatHoleClient))
	//ctl.msgDispatcher.RegisterHandler(&msg.NatHoleReport{}, msg.AsyncHandler(ctl.handleNatHoleReport))
	ctl.msgDispatcher.RegisterHandler(&msg.CloseProxy{}, ctl.handleCloseProxy)
}

func (ctl *Control) handleNewProxy(m msg.Message) {
	xl := ctl.xl
	inMsg := m.(*msg.NewProxy)
	content := &hook.NewProxyContent{
		User: hook.UserInfo{
			User:  ctl.loginMsg.User,
			Metas: ctl.loginMsg.Metas,
			RunID: ctl.loginMsg.RunID,
		},
		NewProxy: *inMsg,
	}
	var remoteAddr string
	retContent, err := ctl.hookManager.NewProxy(content)
	if err == nil {
		inMsg = &retContent.NewProxy
		remoteAddr, err = ctl.RegisterProxy(inMsg)
	}
	// register proxy in this control
	resp := &msg.NewProxyResp{
		ProxyName: inMsg.ProxyName,
	}
	if err != nil {
		xl.Warnf("new proxy [%s] type [%s] error: %v", inMsg.ProxyName, inMsg.ProxyType, err)
		resp.Error = util.GenerateResponseErrorString(fmt.Sprintf("new proxy [%s] error", inMsg.ProxyName),
			err, lo.FromPtr(ctl.serverCfg.DetailedErrorsToClient))
	} else {
		resp.RemoteAddr = remoteAddr
		xl.Infof("new proxy [%s] type [%s] success", inMsg.ProxyName, inMsg.ProxyType)
		metrics.Server.NewProxy(inMsg.ProxyName, inMsg.ProxyType)
	}
	_ = ctl.msgDispatcher.Send(resp)
}

func (ctl *Control) RegisterProxy(pxyMsg *msg.NewProxy) (remoteAddr string, err error) {
	return "", nil
}

func (ctl *Control) handlePing(m msg.Message) {
	xl := ctl.xl
	inMsg := m.(*msg.Ping)

	content := &hook.PingContent{
		User: hook.UserInfo{
			User:  ctl.loginMsg.User,
			Metas: ctl.loginMsg.Metas,
			RunID: ctl.loginMsg.RunID,
		},
		Ping: *inMsg,
	}
	retContent, err := ctl.hookManager.Ping(content)
	if err == nil {
		inMsg = &retContent.Ping
		err = ctl.authVerifier.VerifyPing(inMsg)
	}
	if err != nil {
		xl.Warnf("received invalid ping: %v", err)
		_ = ctl.msgDispatcher.Send(&msg.Pong{
			Error: util.GenerateResponseErrorString("invalid ping", err, lo.FromPtr(ctl.serverCfg.DetailedErrorsToClient)),
		})
		return
	}
	ctl.lastPing.Store(time.Now())
	xl.Debugf("receive heartbeat")
	_ = ctl.msgDispatcher.Send(&msg.Pong{})
}

func (ctl *Control) Start() {
	loginRespMsg := &msg.LoginResp{
		Version: version.Full(),
		RunID:   ctl.runID,
		Error:   "",
	}
	_ = msg.WriteMsg(ctl.conn, loginRespMsg)

	go func() {
		for i := 0; i < ctl.poolCount; i++ {
			// ignore error here, that means that this control is closed
			_ = ctl.msgDispatcher.Send(&msg.ReqWorkConn{})
		}
	}()
	go ctl.worker()
}

func (ctl *Control) Replaced(newCtl *Control) {
	xl := ctl.xl
	xl.Infof("replaced by client [%s]", newCtl.runID)
	ctl.runID = ""
	ctl.conn.Close()
}

func (ctl *Control) RegisterWorkConn(conn net.Conn) error {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("panic error: %v", err)
			log.Errorf(string(debug.Stack()))
		}
	}()

	select {
	case ctl.workConnCh <- conn:
		log.Debugf("new work connection registered")
		return nil
	default:
		log.Debugf("work connection pool is full, discarding")
		return fmt.Errorf("work connection pool is full, discarding")
	}
}

func (ctl *Control) heartbeatWorker() {
	if ctl.serverCfg.Transport.HeartbeatTimeout <= 0 {
		return
	}
	xl := ctl.xl
	go wait.Until(func() {
		if time.Since(ctl.lastPing.Load().(time.Time)) > time.Duration(ctl.serverCfg.Transport.HeartbeatTimeout)*time.Second {
			xl.Warnf("heartbeat timeout")
			ctl.conn.Close()
			return
		}
	}, time.Second, ctl.doneCh)
}

func (ctl *Control) handleCloseProxy(m msg.Message) {
	xl := ctl.xl
	inMsg := m.(*msg.CloseProxy)
	_ = ctl.CloseProxy(inMsg)
	xl.Infof("close proxy [%s] success", inMsg.ProxyName)
}

func (ctl *Control) Close() error {
	ctl.conn.Close()
	return nil
}

func (ctl *Control) WaitClosed() {
	<-ctl.doneCh
}

func (ctl *Control) worker() {

}

func (ctl *Control) CloseProxy(closeMsg *msg.CloseProxy) (err error) {
	ctl.mu.Lock()
	return err
}
