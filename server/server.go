package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/gk7790/gk-zap/pkg/auth"
	m "github.com/gk7790/gk-zap/pkg/config/model"
	hook "github.com/gk7790/gk-zap/pkg/hook/server"
	"github.com/gk7790/gk-zap/pkg/msg"
	pkgNet "github.com/gk7790/gk-zap/pkg/net"
	"github.com/gk7790/gk-zap/pkg/utils/log"
	"github.com/gk7790/gk-zap/pkg/utils/util"
	"github.com/gk7790/gk-zap/pkg/utils/version"
	"github.com/gk7790/gk-zap/pkg/utils/xlog"
	"github.com/gk7790/gk-zap/server/controller"
	cmux "github.com/soheilhy/cmux"
)

const (
	connReadTimeout       time.Duration = 10 * time.Second
	vhostReadWriteTimeout time.Duration = 30 * time.Second
)

// Service Server service
type Service struct {
	// TCP 多协议复用器（区分 HTTP/HTTPS/FRP 等）
	muxer cmux.CMux
	// TCP 主监听器  主端口
	listener net.Listener

	// 服务端配置
	cfg *m.ServerConfig

	hookManager *hook.Manager

	// 资源控制器
	resource *controller.ResourceController

	// 身份认证
	authVerifier auth.Verifier

	// 管理全部 控制连接
	ctlManager *ControlManager

	// 最顶层的“根上下文”
	ctx context.Context
	// 会让所有监听 ctxWithCancel.Done() 的协程退出
	cancel context.CancelFunc
}

func NewService(cfg *m.ServerConfig) (*Service, error) {

	// TODO 这里可以加webserver,

	svr := &Service{
		cfg:         cfg,
		ctlManager:  NewControlManager(),
		hookManager: hook.NewManager(),
		ctx:         context.Background(),
	}

	// Listen for accepting connections from client.
	address := net.JoinHostPort(cfg.BindAddr, strconv.Itoa(cfg.BindPort))
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("create server listener error, %v", err)
	}
	svr.muxer = cmux.New(ln)

	tcpListener := ln.(*net.TCPListener)
	_ = tcpListener.SetDeadline(time.Now().Add(time.Duration(cfg.Transport.TCPKeepAlive) * time.Second))

	// 匹配所有 TCP 流量
	defaultListener := svr.muxer.Match(cmux.Any())

	// 启动 cmux 服务
	go func() {
		if err := svr.muxer.Serve(); err != nil {
			log.Errorf("cmux serve error: %v", err)
		}
	}()
	svr.listener = defaultListener
	log.Infof("gks tcp listen on: %s", address)
	return svr, nil
}

// Run 服务运行
func (svr *Service) Run(ctx context.Context) {
	// 生成一个可取消的 Context
	ctx, cancel := context.WithCancel(ctx)
	svr.ctx = ctx
	svr.cancel = cancel

	svr.HandleListener(svr.listener, false)

	<-svr.ctx.Done()
	// service context may not be canceled by svr.Close(), we should call it here to release resources
	if svr.listener != nil {
		svr.Close()
	}
}

func (svr *Service) Close() error {
	if svr.listener != nil {
		svr.listener.Close()
	}
	svr.muxer.Close()
	return nil
}

// HandleListener 处理监听
func (svr *Service) HandleListener(l net.Listener, internal bool) {
	for {
		c, err := l.Accept()
		if err != nil {
			log.Warnf("listener for incoming connections from client closed")
		}
		ctx := context.Background()

		// 开启一个新的线程处理 connection
		go func(ctx context.Context, frpConn net.Conn) {
			// 判断是否支持 TCP的多路复用器, 并且不是内部
			//if lo.FromPtr(svr.cfg.Transport.TCPMux) && !internal {
			//
			//} else {
			//}
			svr.handleConnection(ctx, frpConn, internal)
		}(ctx, c)
	}
}

// 处理 Connection 链接
func (svr *Service) handleConnection(ctx context.Context, conn net.Conn, internal bool) {
	var (
		rawMsg msg.Message
		err    error
	)

	// 设置读取超时
	_ = conn.SetReadDeadline(time.Now().Add(connReadTimeout))
	rawMsg, err = msg.ReadMsg(conn)
	// 清除 deadline
	_ = conn.SetReadDeadline(time.Time{})

	if err != nil {
		var netErr net.Error
		switch {
		case errors.Is(err, io.EOF):
			log.Warnf("client closed connection", "remote_addr", conn.RemoteAddr())
		case errors.As(err, &netErr) && netErr.Timeout():
			log.Warnf("read timeout", "remote_addr", conn.RemoteAddr())
		default:
			log.Warnf("failed to read message", "remote_addr", conn.RemoteAddr(), "error", err)
		}
		return
	}

	switch m := rawMsg.(type) {
	case *msg.Login:
		// server plugin hook
		content := &hook.LoginContent{
			Login:         *m,
			ClientAddress: conn.RemoteAddr().String(),
		}
		retContent, err := svr.hookManager.Login(content)
		if err == nil {
			m = &retContent.Login
			err = svr.RegisterControl(conn, m, internal)
		}

		// 如果失败, 发送失败响应
		_ = msg.WriteMsg(conn, &msg.LoginResp{
			Version: version.Full(),
			Error:   "验证失败",
		})
	case *msg.NewWorkConn:
		if err := svr.RegisterWorkConn(conn, m); err != nil {
			conn.Close()
		}
	case *msg.NewVisitorConn:
		if err = svr.RegisterVisitorConn(conn, m); err != nil {
			log.Warnf("register visitor conn error: %v", err)
			_ = msg.WriteMsg(conn, &msg.NewVisitorConnResp{
				ProxyName: m.ProxyName,
				Error:     "register visitor conn error",
			})
			conn.Close()
		} else {
			_ = msg.WriteMsg(conn, &msg.NewVisitorConnResp{
				ProxyName: m.ProxyName,
				Error:     "",
			})
		}
	default:
		log.Warnf("error message type for the new connection [%s]", conn.RemoteAddr().String())
		conn.Close()
	}
}

// RegisterControl 负责注册控制连接的核心逻辑
func (svr *Service) RegisterControl(ctlConn net.Conn, loginMsg *msg.Login, internal bool) error {
	// 1. 生成唯一 RunID
	if loginMsg.RunID == "" {
		id, err := util.RandID()
		if err != nil {
			return err
		}
		loginMsg.RunID = id
	}

	ctx := pkgNet.NewContextFromConn(ctlConn)
	xl := xlog.FromContextSafe(ctx)
	xl.AppendPrefix(loginMsg.RunID)
	ctx = xlog.NewContext(ctx, xl)

	log.Infof("client login info: ip [%s] version [%s] hostname [%s] os [%s] arch [%s]",
		ctlConn.RemoteAddr().String(), loginMsg.Version, loginMsg.Hostname, loginMsg.Os, loginMsg.Arch)

	// 3. 校验认证
	//authVerifier := svr.authVerifier
	//if internal && loginMsg.ClientSpec.AlwaysAuthPass {
	//	authVerifier = auth.AlwaysPassVerifier
	//}
	//if err := authVerifier.VerifyLogin(loginMsg); err != nil {
	//	return err
	//}

	// 4. 创建新的控制器
	ctl, err := NewControl(ctx, ctlConn, svr.hookManager, loginMsg, svr.cfg)
	if err != nil {
		log.Warnf("create new controller error: %v", err)
		return fmt.Errorf("unexpected error when creating new controller")
	}

	// 5. 替换旧控制器（同 RunID）
	if oldCtl := svr.ctlManager.Add(loginMsg.RunID, ctl); oldCtl != nil {
		oldCtl.WaitClosed()
	}

	// 6. 启动控制器
	ctl.Start()

	// 7. 异步清理关闭的控制器
	go func() {
		// block until control closed
		ctl.WaitClosed()
		svr.ctlManager.Del(loginMsg.RunID, ctl)
	}()

	return err
}

// RegisterWorkConn 注册 Work Conn（工作连接）
func (svr *Service) RegisterWorkConn(workConn net.Conn, newMsg *msg.NewWorkConn) error {
	ctl, exist := svr.ctlManager.GetByID(newMsg.RunID)
	if !exist {
		log.Warnf("no client control found for run id [%s]", newMsg.RunID)
		return fmt.Errorf("no client control found for run id [%s]", newMsg.RunID)
	}
	// server plugin hook
	content := &hook.NewWorkConnContent{
		User: hook.UserInfo{
			User:  ctl.loginMsg.User,
			Metas: ctl.loginMsg.Metas,
			RunID: ctl.loginMsg.RunID,
		},
		NewWorkConn: *newMsg,
	}
	retContent, err := svr.hookManager.NewWorkConn(content)
	if err == nil {
		newMsg = &retContent.NewWorkConn
		// Check auth.
		err = ctl.authVerifier.VerifyNewWorkConn(newMsg)
	}
	if err != nil {
		log.Warnf("invalid NewWorkConn with run id [%s]", newMsg.RunID)
		_ = msg.WriteMsg(workConn, &msg.StartWorkConn{
			Error: "invalid NewWorkConn",
		})
		return fmt.Errorf("invalid NewWorkConn with run id [%s]", newMsg.RunID)
	}
	return ctl.RegisterWorkConn(workConn)
}

// RegisterVisitorConn 注册处理 Visitor Conn（访客连接）
func (svr *Service) RegisterVisitorConn(visitorConn net.Conn, newMsg *msg.NewVisitorConn) error {
	visitorUser := ""
	if newMsg.RunID != "" {
		ctl, exist := svr.ctlManager.GetByID(newMsg.RunID)
		if !exist {
			return fmt.Errorf("no client control found for run id [%s]", newMsg.RunID)
		}
		visitorUser = ctl.loginMsg.User
	}
	return svr.resource.VisitorManager.NewConn(visitorConn, newMsg, visitorUser)
}
