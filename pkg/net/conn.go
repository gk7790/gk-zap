package net

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// -------------------------------
// ContextConn: 连接上下文扩展
// -------------------------------

type ContextGetter interface {
	Context() context.Context
}

type ContextSetter interface {
	WithContext(ctx context.Context)
}

type ContextConn struct {
	net.Conn
	ctx context.Context
}

func NewContextConn(ctx context.Context, c net.Conn) *ContextConn {
	return &ContextConn{
		Conn: c,
		ctx:  ctx,
	}
}

func NewContextFromConn(conn net.Conn) context.Context {
	if c, ok := conn.(ContextGetter); ok {
		return c.Context()
	}
	return context.Background()
}

func (c *ContextConn) WithContext(ctx context.Context) { c.ctx = ctx }

func (c *ContextConn) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// -------------------------------
// WrapReadWriteCloserConn: 将 io.ReadWriteCloser 封装成 net.Conn
// -------------------------------

type WrapReadWriteCloserConn struct {
	io.ReadWriteCloser
	underConn  net.Conn
	remoteAddr net.Addr
}

func WrapReadWriteCloserToConn(rwc io.ReadWriteCloser, underConn net.Conn) *WrapReadWriteCloserConn {
	return &WrapReadWriteCloserConn{
		ReadWriteCloser: rwc,
		underConn:       underConn,
	}
}

func (conn *WrapReadWriteCloserConn) LocalAddr() net.Addr {
	if conn.underConn != nil {
		return conn.underConn.LocalAddr()
	}
	return (*net.TCPAddr)(nil)
}

func (conn *WrapReadWriteCloserConn) SetRemoteAddr(addr net.Addr) { conn.remoteAddr = addr }

func (conn *WrapReadWriteCloserConn) RemoteAddr() net.Addr {
	if conn.remoteAddr != nil {
		return conn.remoteAddr
	}
	if conn.underConn != nil {
		return conn.underConn.RemoteAddr()
	}
	return (*net.TCPAddr)(nil)
}

func (conn *WrapReadWriteCloserConn) SetDeadline(t time.Time) error {
	if conn.underConn != nil {
		return conn.underConn.SetDeadline(t)
	}
	return &net.OpError{Op: "set", Net: "wrap", Err: errors.New("deadline not supported")}
}

func (conn *WrapReadWriteCloserConn) SetReadDeadline(t time.Time) error {
	if conn.underConn != nil {
		return conn.underConn.SetReadDeadline(t)
	}
	return &net.OpError{Op: "set", Net: "wrap", Err: errors.New("deadline not supported")}
}

func (conn *WrapReadWriteCloserConn) SetWriteDeadline(t time.Time) error {
	if conn.underConn != nil {
		return conn.underConn.SetWriteDeadline(t)
	}
	return &net.OpError{Op: "set", Net: "wrap", Err: errors.New("deadline not supported")}
}

// -------------------------------
// CloseNotifyConn: 带关闭通知（修复了递归调用问题）
// -------------------------------

type CloseNotifyConn struct {
	net.Conn
	closeFlag int32
	closeFn   func()
}

func WrapCloseNotifyConn(c net.Conn, closeFn func()) *CloseNotifyConn {
	return &CloseNotifyConn{
		Conn:    c,
		closeFn: closeFn,
	}
}

func (cc *CloseNotifyConn) Close() error {
	if atomic.SwapInt32(&cc.closeFlag, 1) == 0 {
		// 调用底层 Conn 的 Close（不是自己）
		err := cc.Conn.Close()
		if cc.closeFn != nil {
			cc.closeFn()
		}
		return err
	}
	return nil
}

// -------------------------------
// StatsConn: 统计读写字节数
// -------------------------------

type StatsConn struct {
	net.Conn
	totalRead  int64
	totalWrite int64
	closed     int64
	statsFunc  func(totalRead, totalWrite int64)
}

func WrapStatsConn(conn net.Conn, statsFunc func(totalRead, totalWrite int64)) *StatsConn {
	return &StatsConn{
		Conn:      conn,
		statsFunc: statsFunc,
	}
}

func (s *StatsConn) Read(p []byte) (n int, err error) {
	n, err = s.Conn.Read(p)
	if n > 0 {
		atomic.AddInt64(&s.totalRead, int64(n))
	}
	return
}

func (s *StatsConn) Write(p []byte) (n int, err error) {
	n, err = s.Conn.Write(p)
	if n > 0 {
		atomic.AddInt64(&s.totalWrite, int64(n))
	}
	return
}

func (s *StatsConn) Close() error {
	if atomic.SwapInt64(&s.closed, 1) == 0 {
		err := s.Conn.Close()
		if s.statsFunc != nil {
			s.statsFunc(atomic.LoadInt64(&s.totalRead), atomic.LoadInt64(&s.totalWrite))
		}
		return err
	}
	return nil
}

// -------------------------------
// Crypto ReadWriter using AES-GCM with simple framing
// - Uses standard library only
// - Frame format: [4-byte big-endian length][nonce (nonceSize)][ciphertext]
// - Each Write call is treated as one message/frame (so callers should manage chunking as desired)
// - Write is protected by a mutex to avoid interleaving frames
// -------------------------------

type cryptoRW struct {
	rw        io.ReadWriter
	aead      cipher.AEAD
	nonceSize int
	writeMu   sync.Mutex
	readBuf   []byte // temporary buffer for read frame
}

// NewCryptoReadWriter returns an io.ReadWriter that encrypts writes and decrypts reads.
// key must be 16, 24, or 32 bytes (AES-128/192/256).
// Note: This implementation treats each Write() call as a single message/frame.
// For large streams, the caller should frame application-level data appropriately.
func NewCryptoReadWriter(rw io.ReadWriter, key []byte) (io.ReadWriter, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("crypto: key must be 16, 24 or 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &cryptoRW{
		rw:        rw,
		aead:      aead,
		nonceSize: aead.NonceSize(),
	}, nil
}

// Write: encrypts the provided plaintext as one frame and writes:
// [4-byte length][nonce][ciphertext]
func (c *cryptoRW) Write(p []byte) (n int, err error) {
	// ensure whole frame written atomically relative to other Write() callers
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	nonce := make([]byte, c.nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return 0, err
	}
	ciphertext := c.aead.Seal(nil, nonce, p, nil)
	// frame is nonce + ciphertext
	frameLen := uint32(len(nonce) + len(ciphertext))
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], frameLen)

	// write length, nonce, ciphertext
	if _, err := c.rw.Write(lenBuf[:]); err != nil {
		return 0, err
	}
	if _, err := c.rw.Write(nonce); err != nil {
		return 0, err
	}
	if _, err := c.rw.Write(ciphertext); err != nil {
		return 0, err
	}
	// return number of plaintext bytes considered written
	return len(p), nil
}

// Read: reads a frame and returns decrypted plaintext
func (c *cryptoRW) Read(p []byte) (int, error) {
	// read 4-byte length
	var lenBuf [4]byte
	if _, err := io.ReadFull(c.rw, lenBuf[:]); err != nil {
		return 0, err
	}
	frameLen := binary.BigEndian.Uint32(lenBuf[:])
	if frameLen < uint32(c.nonceSize) {
		return 0, errors.New("crypto: invalid frame length")
	}
	toRead := int(frameLen)
	buf := make([]byte, toRead)
	if _, err := io.ReadFull(c.rw, buf); err != nil {
		return 0, err
	}
	nonce := buf[:c.nonceSize]
	ciphertext := buf[c.nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return 0, err
	}
	// copy into supplied buffer
	n := copy(p, plaintext)
	if n < len(plaintext) {
		// If user's p is smaller than plaintext, we return the first part and keep remainder?
		// Simpler: return io.ErrShortBuffer to indicate buffer too small.
		// Caller should provide a large enough buffer or read in loop.
		return n, io.ErrShortBuffer
	}
	return n, nil
}
