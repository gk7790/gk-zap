package auth

import (
	"fmt"
	"slices"
	"time"

	m "github.com/gk7790/gk-zap/pkg/config/model"
	"github.com/gk7790/gk-zap/pkg/msg"
	"github.com/gk7790/gk-zap/pkg/utils/util"
)

type TokenAuthSetterVerifier struct {
	additionalAuthScopes []m.AuthScope
	token                string
}

func NewTokenAuth(additionalAuthScopes []m.AuthScope, token string) *TokenAuthSetterVerifier {
	return &TokenAuthSetterVerifier{
		additionalAuthScopes: additionalAuthScopes,
		token:                token,
	}
}

func (auth *TokenAuthSetterVerifier) SetLogin(loginMsg *msg.Login) error {
	loginMsg.PrivilegeKey = util.GetAuthKey(auth.token, loginMsg.Timestamp)
	return nil
}

func (auth *TokenAuthSetterVerifier) SetPing(pingMsg *msg.Ping) error {
	if !slices.Contains(auth.additionalAuthScopes, m.AuthScopeHeartBeats) {
		return nil
	}

	pingMsg.Timestamp = time.Now().Unix()
	pingMsg.PrivilegeKey = util.GetAuthKey(auth.token, pingMsg.Timestamp)
	return nil
}

func (auth *TokenAuthSetterVerifier) SetNewWorkConn(newWorkConnMsg *msg.NewWorkConn) error {
	if !slices.Contains(auth.additionalAuthScopes, m.AuthScopeNewWorkConns) {
		return nil
	}

	newWorkConnMsg.Timestamp = time.Now().Unix()
	newWorkConnMsg.PrivilegeKey = util.GetAuthKey(auth.token, newWorkConnMsg.Timestamp)
	return nil
}

func (auth *TokenAuthSetterVerifier) VerifyLogin(m *msg.Login) error {
	if !util.ConstantTimeEqString(util.GetAuthKey(auth.token, m.Timestamp), m.PrivilegeKey) {
		return fmt.Errorf("token in login doesn't match token from configuration")
	}
	return nil
}

func (auth *TokenAuthSetterVerifier) VerifyPing(msg *msg.Ping) error {
	if !slices.Contains(auth.additionalAuthScopes, m.AuthScopeHeartBeats) {
		return nil
	}

	if !util.ConstantTimeEqString(util.GetAuthKey(auth.token, msg.Timestamp), msg.PrivilegeKey) {
		return fmt.Errorf("token in heartbeat doesn't match token from configuration")
	}
	return nil
}

func (auth *TokenAuthSetterVerifier) VerifyNewWorkConn(msg *msg.NewWorkConn) error {
	if !slices.Contains(auth.additionalAuthScopes, m.AuthScopeNewWorkConns) {
		return nil
	}

	if !util.ConstantTimeEqString(util.GetAuthKey(auth.token, msg.Timestamp), msg.PrivilegeKey) {
		return fmt.Errorf("token in NewWorkConn doesn't match token from configuration")
	}
	return nil
}
