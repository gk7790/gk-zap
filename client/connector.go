package client

import (
	"context"
	"crypto/tls"
	"strconv"

	m "github.com/gk7790/gk-zap/pkg/config/model"
	fmux "github.com/hashicorp/yamux"
	quic "github.com/quic-go/quic-go"
	"github.com/samber/lo"

	"net"
	"strings"
	"sync"
	"time"
)

type Connector interface {
	Open() error
	Connect() (net.Conn, error)
	Close() error
}

type simpleConnector struct {
	ctx        context.Context
	cfg        *m.ClientCommonConfig
	muxSession *fmux.Session
	quicConn   *quic.Conn
	closeOnce  sync.Once
}

func NewConnector(ctx context.Context, cfg *m.ClientCommonConfig) Connector {
	return &simpleConnector{
		ctx: ctx,
		cfg: cfg,
	}
}

func (c *simpleConnector) Open() error {
	if strings.EqualFold(c.cfg.Transport.Protocol, "quic") {
		var tlsConfig *tls.Config
		var err error
		sn := c.cfg.Transport.TLS.ServerName
		if sn == "" {
			sn = c.cfg.ServerAddr
		}

		conn, err := quic.DialAddr(
			c.ctx,
			net.JoinHostPort(c.cfg.ServerAddr, strconv.Itoa(c.cfg.ServerPort)),
			tlsConfig, &quic.Config{
				MaxIdleTimeout:     time.Duration(c.cfg.Transport.QUIC.MaxIdleTimeout) * time.Second,
				MaxIncomingStreams: int64(c.cfg.Transport.QUIC.MaxIncomingStreams),
				KeepAlivePeriod:    time.Duration(c.cfg.Transport.QUIC.KeepalivePeriod) * time.Second,
			})
		if err != nil {
			return err
		}
		c.quicConn = conn
	}
	if !lo.FromPtr(c.cfg.Transport.TCPMux) {
		return nil
	}

	return nil
}

func (c *simpleConnector) Connect() (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(c.ctx, "tcp", "")
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *simpleConnector) Close() error {
	c.closeOnce.Do(func() {
		if c.quicConn != nil {
			_ = c.quicConn.CloseWithError(0, "")
		}
		if c.muxSession != nil {
			_ = c.muxSession.Close()
		}
	})
	return nil
}
