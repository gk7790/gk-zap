package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	m "github.com/gk7790/gk-zap/pkg/config/model"
	"github.com/gk7790/gk-zap/pkg/msg"
	"github.com/samber/lo"
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

	// 最顶层的“根上下文”
	ctx context.Context
	// 会让所有监听 ctxWithCancel.Done() 的协程退出
	cancel context.CancelFunc
}

func NewService(cfg *m.ServerConfig) (*Service, error) {

	// TODO 这里可以加webserver,

	svr := &Service{
		cfg: cfg,
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
			slog.Error("cmux serve error: %v", err)
		}
	}()
	svr.listener = defaultListener
	slog.Info("gks tcp listen on %s", address)

	return svr, nil
}

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

func (svr *Service) HandleListener(l net.Listener, internal bool) {
	for {
		c, err := l.Accept()
		if err != nil {
			slog.Warn("listener for incoming connections from client closed")
		}
		ctx := context.Background()

		// 开启一个新的线程处理 connection
		go func(ctx context.Context, frpConn net.Conn) {
			// 判断是否支持 TCP的多路复用器, 并且不是内部
			if lo.FromPtr(svr.cfg.Transport.TCPMux) && !internal {

			} else {
				svr.handleConnection(ctx, frpConn, internal)
			}
		}(ctx, c)
	}
}

func (svr *Service) handleConnection(ctx context.Context, conn net.Conn, internal bool) {
	defer conn.Close()
	// 设置读取超时
	_ = conn.SetReadDeadline(time.Now().Add(connReadTimeout))
	rawMsg, err := msg.ReadMsg(conn)
	// 清除 deadline
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		var netErr net.Error
		switch {
		case errors.Is(err, io.EOF):
			slog.Warn("client closed connection", "remote_addr", conn.RemoteAddr())
		case errors.As(err, &netErr) && netErr.Timeout():
			slog.Warn("read timeout", "remote_addr", conn.RemoteAddr())
		default:
			slog.Warn("failed to read message", "remote_addr", conn.RemoteAddr(), "error", err)
		}
		return
	}
	slog.Info("Received message: %#v", rawMsg)

}
