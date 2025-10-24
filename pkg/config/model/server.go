package model

import (
	"github.com/gk7790/gk-zap/pkg/utils/value"
	"github.com/samber/lo"
)

type ServerConfig struct {
	Version  string `json:"version"`
	BindAddr string `json:"bindAddr,omitempty"`
	BindPort int    `json:"bindPort,omitempty"`

	// 设置各种连接的配置, 保活时间
	Transport ServerTransportConfig `json:"transport,omitempty"`
}

func (c *ServerConfig) Complete() error {
	c.Transport.Complete()
	c.BindAddr = value.EmptyOr(c.BindAddr, "0.0.0.0")
	c.BindPort = value.EmptyOr(c.BindPort, 7000)
	return nil
}

type ServerTransportConfig struct {
	// TCPMux toggles TCP stream multiplexing. This allows multiple requests
	// from a client to share a single TCP connection. By default, this value
	// is true.
	// $HideFromDoc
	TCPMux *bool `json:"tcpMux,omitempty"`
	// TCPMuxKeepaliveInterval specifies the keep alive interval for TCP stream multiplier.
	// If TCPMux is true, heartbeat of application layer is unnecessary because it can only rely on heartbeat in TCPMux.
	TCPMuxKeepaliveInterval int64 `json:"tcpMuxKeepaliveInterval,omitempty"`
	// TCPKeepAlive specifies the interval between keep-alive probes for an active network connection between frpc and frps.
	// If negative, keep-alive probes are disabled.
	TCPKeepAlive int64 `json:"tcpKeepalive,omitempty"`
	// MaxPoolCount specifies the maximum pool size for each proxy. By default,
	// this value is 5.
	MaxPoolCount int64 `json:"maxPoolCount,omitempty"`
	// HeartBeatTimeout specifies the maximum time to wait for a heartbeat
	// before terminating the connection. It is not recommended to change this
	// value. By default, this value is 90. Set negative value to disable it.
	HeartbeatTimeout int64 `json:"heartbeatTimeout,omitempty"`
}

func (c *ServerTransportConfig) Complete() {
	c.TCPMux = value.EmptyOr(c.TCPMux, lo.ToPtr(true))
	c.TCPMuxKeepaliveInterval = value.EmptyOr(c.TCPMuxKeepaliveInterval, 30)
	c.TCPKeepAlive = value.EmptyOr(c.TCPKeepAlive, 7200)
	c.MaxPoolCount = value.EmptyOr(c.MaxPoolCount, 5)
	if lo.FromPtr(c.TCPMux) {
		// If TCPMux is enabled, heartbeat of application layer is unnecessary because we can rely on heartbeat in tcpmux.
		c.HeartbeatTimeout = value.EmptyOr(c.HeartbeatTimeout, -1)
	} else {
		c.HeartbeatTimeout = value.EmptyOr(c.HeartbeatTimeout, 90)
	}
}
