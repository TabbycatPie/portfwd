package main

import (
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func nowUnix() int64 { return time.Now().Unix() }

// ---------- 连接事件 ----------

type ConnEvent struct {
	At     time.Time
	Rule   string
	Client string
	Target string
	Bytes  int64
	Dur    time.Duration
	Err    string
	Open   bool
}

var (
	evMu   sync.Mutex
	events []*ConnEvent
)

const maxEvents = 300

// notifyUI 由 UI 层设置：有新事件时触发重绘
var notifyUI = func() {}

func recordOpen(rule, client, target string) *ConnEvent {
	e := &ConnEvent{At: time.Now(), Rule: rule, Client: client, Target: target, Open: true}
	evMu.Lock()
	events = append(events, e)
	if len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}
	evMu.Unlock()
	notifyUI()
	return e
}

func recordClose(e *ConnEvent, n int64, err error) {
	evMu.Lock()
	e.Bytes = n
	e.Dur = time.Since(e.At)
	e.Open = false
	if err != nil {
		e.Err = err.Error()
	}
	evMu.Unlock()
	notifyUI()
}

// eventsSnapshot 返回倒序副本（最新在前）
func eventsSnapshot() []ConnEvent {
	evMu.Lock()
	defer evMu.Unlock()
	out := make([]ConnEvent, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		out = append(out, *events[i])
	}
	return out
}

func clearEvents() {
	evMu.Lock()
	events = nil
	evMu.Unlock()
	notifyUI()
}

// ---------- 转发引擎 ----------

type fwdState struct {
	ln     net.Listener
	err    string
	conns  int64
	active int32
}

type Engine struct {
	mu    sync.Mutex
	state map[string]*fwdState
}

func NewEngine() *Engine { return &Engine{state: map[string]*fwdState{}} }

func (e *Engine) Running(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := e.state[id]
	return st != nil && st.ln != nil
}

func (e *Engine) Err(id string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if st := e.state[id]; st != nil {
		return st.err
	}
	return ""
}

func (e *Engine) Conns(id string) int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if st := e.state[id]; st != nil {
		return atomic.LoadInt64(&st.conns)
	}
	return 0
}

func (e *Engine) Start(it Item) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if st := e.state[it.ID]; st != nil && st.ln != nil {
		return nil
	}
	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(it.LocalPort))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		e.state[it.ID] = &fwdState{err: err.Error()}
		logf("[fwd] start FAIL rule=%q listen %s err=%v", it.Name, addr, err)
		return err
	}
	logf("[fwd] start ok rule=%q listen %s -> %s:%d", it.Name, addr, it.TargetHost, it.TargetPort)
	st := &fwdState{ln: ln}
	e.state[it.ID] = st
	go e.acceptLoop(it, st)
	return nil
}

// 双向计数的连接包装
type countingConn struct {
	net.Conn
	n *int64
}

func (c countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		atomic.AddInt64(c.n, int64(n))
	}
	return n, err
}

func (c countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		atomic.AddInt64(c.n, int64(n))
	}
	return n, err
}

func (e *Engine) acceptLoop(it Item, st *fwdState) {
	target := net.JoinHostPort(it.TargetHost, strconv.Itoa(it.TargetPort))
	rule := it.Name
	if rule == "" {
		rule = target
	}
	for {
		c, err := st.ln.Accept()
		if err != nil {
			e.mu.Lock()
			if st.ln != nil {
				st.err = ""
				st.ln = nil
			}
			e.mu.Unlock()
			return
		}
		atomic.AddInt64(&st.conns, 1)
		go func(src net.Conn) {
			defer src.Close()
			client := src.RemoteAddr().String()
			ev := recordOpen(rule, client, target)
			logf("[conn] open rule=%q from=%s to=%s", rule, client, target)

			var total int64
			dst, derr := net.DialTimeout("tcp", target, 6*time.Second)
			if derr != nil {
				e.mu.Lock()
				st.err = T("err.unreachable") + derr.Error()
				e.mu.Unlock()
				recordClose(ev, 0, derr)
				logf("[conn] dial FAIL from=%s to=%s err=%v", client, target, derr)
				return
			}
			defer dst.Close()

			cs := countingConn{Conn: src, n: &total}
			cd := countingConn{Conn: dst, n: &total}
			done := make(chan struct{}, 2)
			go func() { _, _ = io.Copy(cd, cs); done <- struct{}{} }()
			go func() { _, _ = io.Copy(cs, cd); done <- struct{}{} }()
			<-done
			n := atomic.LoadInt64(&total)
			recordClose(ev, n, nil)
			logf("[conn] close from=%s to=%s bytes=%d dur=%s", client, target, n, time.Since(ev.At).Round(time.Millisecond))
		}(c)
	}
}

func (e *Engine) Stop(id string) {
	e.mu.Lock()
	st := e.state[id]
	if st != nil {
		ln := st.ln
		st.ln = nil
		st.err = ""
		e.mu.Unlock()
		logf("[fwd] stop id=%s", id)
		if ln != nil {
			_ = ln.Close()
		}
		return
	}
	e.mu.Unlock()
}

func (e *Engine) StopAll(items []Item) {
	for _, it := range items {
		e.Stop(it.ID)
	}
}

var errNoPort = errors.New("端口无效")
