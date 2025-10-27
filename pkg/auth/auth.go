package auth

import (
	"fmt"

	m "github.com/gk7790/gk-zap/pkg/config/model"
	"github.com/gk7790/gk-zap/pkg/msg"
)

type Setter interface {
	SetLogin(*msg.Login) error
	SetPing(*msg.Ping) error
	SetNewWorkConn(*msg.NewWorkConn) error
}

func NewAuthSetter(cfg m.AuthClientConfig) (authProvider Setter, err error) {
	switch cfg.Method {
	case m.AuthMethodToken:
		authProvider = NewTokenAuth(cfg.AdditionalScopes, cfg.Token)
	case m.AuthMethodOIDC:
		// TODO
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", cfg.Method)
	}
	return authProvider, nil
}

type Verifier interface {
	VerifyLogin(*msg.Login) error
	VerifyPing(*msg.Ping) error
	VerifyNewWorkConn(*msg.NewWorkConn) error
}

func NewAuthVerifier(cfg m.AuthServerConfig) (authVerifier Verifier) {
	switch cfg.Method {
	case m.AuthMethodToken:
		authVerifier = NewTokenAuth(cfg.AdditionalScopes, cfg.Token)
	case m.AuthMethodOIDC:
		// TODO
	}
	return authVerifier
}
