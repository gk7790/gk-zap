// Copyright 2023 The frp Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package msg

import (
	"io"
	"reflect"
)

// AsyncHandler 把一个普通的处理函数 包装成异步执行版本
func AsyncHandler(f func(Message)) func(Message) {
	return func(m Message) {
		go f(m)
	}
}

// Dispatcher 消息调度器
//
//	1.持续读取消息 → 找到对应的 handler → 调用。
//	2.发送消息 → 异步通过 sendLoop 写入连接。
//	3.支持关闭、退出等控制。
type Dispatcher struct {
	// 网络连接或任何可读写的流
	rw io.ReadWriter
	// 发送队列
	sendCh chan Message
	// 退出信号
	doneCh chan struct{}
	// 消息类型到处理函数的映射
	msgHandlers map[reflect.Type]func(Message)
	// 默认处理函数
	defaultHandler func(Message)
}

func NewDispatcher(rw io.ReadWriter) *Dispatcher {
	return &Dispatcher{
		rw:          rw,
		sendCh:      make(chan Message, 100),
		doneCh:      make(chan struct{}),
		msgHandlers: make(map[reflect.Type]func(Message)),
	}
}

// Run 启动两个 goroutine：
//
//	1.sendLoop: 负责写出消息；
//	2.readLoop: 负责读取消息并分发。
//
// 这两个循环通常会在连接建立后立即启动。
func (d *Dispatcher) Run() {
	go d.sendLoop()
	go d.readLoop()
}

// 不断监听 sendCh；
// 收到消息后调用 WriteMsg（把 Message 编码写入连接）；
// 如果关闭信号出现 (doneCh)，退出循环。
func (d *Dispatcher) sendLoop() {
	for {
		select {
		case <-d.doneCh:
			return
		case m := <-d.sendCh:
			_ = WriteMsg(d.rw, m)
		}
	}
}

// 调用 ReadMsg 读取并解析消息；
//
//	1.根据消息的具体类型（reflect.TypeOf(m)）找到对应处理器；
//	2.找不到则交给 defaultHandler；
//	3.如果发生错误（例如连接断开），关闭 doneCh，让其他循环也退出。
func (d *Dispatcher) readLoop() {
	for {
		m, err := ReadMsg(d.rw)
		if err != nil {
			close(d.doneCh)
			return
		}

		if handler, ok := d.msgHandlers[reflect.TypeOf(m)]; ok {
			handler(m)
		} else if d.defaultHandler != nil {
			d.defaultHandler(m)
		}
	}
}

// Send 异步发送一条消息：
//
//	1.如果连接已关闭 (doneCh 关闭)，返回 io.EOF；
//	2.否则把消息放进 sendCh，等待 sendLoop 写出。
func (d *Dispatcher) Send(m Message) error {
	select {
	case <-d.doneCh:
		return io.EOF
	case d.sendCh <- m:
		return nil
	}
}

func (d *Dispatcher) SendChannel() chan Message {
	return d.sendCh
}

// RegisterHandler 注册一个处理函数：不同消息结构体可以注册不同函数。
func (d *Dispatcher) RegisterHandler(msg Message, handler func(Message)) {
	d.msgHandlers[reflect.TypeOf(msg)] = handler
}

// RegisterDefaultHandler 注册默认处理器，当找不到对应类型的 handler 时会执行
func (d *Dispatcher) RegisterDefaultHandler(handler func(Message)) {
	d.defaultHandler = handler
}

// Done 暴露出内部的退出信号
func (d *Dispatcher) Done() chan struct{} {
	return d.doneCh
}
