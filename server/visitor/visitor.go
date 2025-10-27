package visitor

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/gk7790/gk-zap/pkg/msg"
	pkgNet "github.com/gk7790/gk-zap/pkg/net"
)

type listenerBundle struct {
	l          *pkgNet.InternalListener
	sk         string
	allowUsers []string
}

type Manager struct {
	listeners map[string]*listenerBundle

	mu sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		listeners: make(map[string]*listenerBundle),
	}
}

func (mg *Manager) Listen(name string, sk string, allowUsers []string) (*pkgNet.InternalListener, error) {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	if _, ok := mg.listeners[name]; ok {
		return nil, fmt.Errorf("custom listener for [%s] is repeated", name)
	}

	l := pkgNet.NewInternalListener()
	mg.listeners[name] = &listenerBundle{
		l:          l,
		sk:         sk,
		allowUsers: allowUsers,
	}
	return l, nil
}

func (mg *Manager) NewConn(conn net.Conn, newMsg *msg.NewVisitorConn, visitorUser string) (err error) {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	name := newMsg.ProxyName

	if l, ok := mg.listeners[name]; ok {
		// 验证连接签名是否正确
		//if util.GetAuthKey(l.sk, newMsg.Timestamp) != newMsg.SignKey {
		//	err = fmt.Errorf("visitor connection of [%s] auth failed", name)
		//	return
		//}
		// 检查访问权限
		//if !slices.Contains(l.allowUsers, visitorUser) && !slices.Contains(l.allowUsers, "*") {
		//	err = fmt.Errorf("visitor connection of [%s] user [%s] not allowed", name, visitorUser)
		//	return
		//}

		var rwc io.ReadWriteCloser = conn
		// 如果启用了加密（`useEncryption=true`）
		if newMsg.UseEncryption {
			// TODO 加密处理
		}
		// 控制是否对传输的数据进行压缩
		if newMsg.UseCompression {
			// TODO 控制是否对传输的数据进行压缩
		}

		err = l.l.PutConn(pkgNet.WrapReadWriteCloserToConn(rwc, conn))
	} else {
		err = fmt.Errorf("custom listener for [%s] doesn't exist", name)
		return
	}
	return
}

func (mg *Manager) CloseListener(name string) {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	delete(mg.listeners, name)
}
