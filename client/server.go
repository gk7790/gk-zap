package client

import (
	"context"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/gk7790/gk-zap/pkg/auth"
	m "github.com/gk7790/gk-zap/pkg/config/model"
	"github.com/gk7790/gk-zap/pkg/msg"
	"github.com/gk7790/gk-zap/pkg/utils/version"
)

type cancelErr struct {
	Err error
}

func (e cancelErr) Error() string {
	return e.Err.Error()
}

type ServiceOptions struct {
	Common           *m.ClientCommonConfig
	ConfigFilePath   string
	ClientSpec       *msg.ClientSpec
	ConnectorCreator func(context.Context, *m.ClientCommonConfig) Connector
}

type SessionContext struct {
	Common *m.ClientCommonConfig
}

type Control struct {
	ctx context.Context
	// session context
	sessionCtx *SessionContext

	doneCh chan struct{}
}

func NewControl(ctx context.Context, sessionCtx *SessionContext) (*Control, error) {
	ctl := &Control{
		ctx:        ctx,
		sessionCtx: sessionCtx,
		doneCh:     make(chan struct{}),
	}

	return ctl, nil
}

func setServiceOptionsDefault(options *ServiceOptions) error {
	if options.Common != nil {
		if err := options.Common.Complete(); err != nil {
			return err
		}
	}
	if options.ConnectorCreator == nil {
		options.ConnectorCreator = NewConnector
	}
	return nil
}

type Service struct {
	// Uniq id got from frps, it will be attached to loginMsg.
	runID string
	// 服务上下文
	ctx context.Context
	// 异步复用
	ctlMu sync.RWMutex

	authSetter auth.Setter
	// manager control connection with server
	ctl              *Control
	cfgMu            sync.RWMutex
	common           *m.ClientCommonConfig
	clientSpec       *msg.ClientSpec
	connectorCreator func(context.Context, *m.ClientCommonConfig) Connector

	cancel context.CancelCauseFunc
}

func NewService(options ServiceOptions) (*Service, error) {
	ccc := &m.ClientCommonConfig{
		ServerAddr: "127.0.0.1",
		ServerPort: 7000,
	}

	s := &Service{
		ctx:    context.Background(),
		common: ccc,
	}

	return s, nil
}

func (svr *Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	svr.cancel = cancel
	//connector := svr.connectorCreator(svr.ctx, svr.common)
	//conn, err := connector.Connect()
	//if err != nil {
	//	return nil
	//}

	// 1️⃣ 连接服务器
	conn, err := net.Dial("tcp", "127.0.0.1:7000")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	loginMsg := &msg.Login{
		Arch:         runtime.GOARCH,
		Os:           runtime.GOOS,
		PoolCount:    1,
		User:         svr.common.User,
		Version:      version.Full(),
		Timestamp:    time.Now().Unix(),
		RunID:        svr.runID,
		PrivilegeKey: "94b1c59dae033669a7bdce57cccc6a99",
	}
	if err = msg.WriteMsg(conn, loginMsg); err != nil {
		return nil
	}

	//<-svr.ctx.Done()
	//svr.stop()
	return nil
}

func (svr *Service) stop() {
	svr.ctlMu.Lock()
	defer svr.ctlMu.Unlock()
	if svr.ctl != nil {
		svr.ctl = nil
	}
}
