package ipc

import (
	"fmt"
	"sync"
	"time"
)

type IpcClientPool struct {
	serverName       string
	maxConns         int
	idleTimeout      uint32
	connTimeout      uint32
	rwTimeout        uint32
	mu               sync.Mutex
	idleConns        []*IpcConnection
	activeCount      int
}

func NewIpcClientPool(serverName string, maxConns int, idleTimeout, connTimeout, rwTimeout uint32) *IpcClientPool {
	if maxConns <= 0 {
		maxConns = IpcDefaultMaxConnections
	}
	if idleTimeout == 0 {
		idleTimeout = IpcDefaultIdleTimeoutMs
	}
	if connTimeout == 0 {
		connTimeout = IpcDefaultTimeoutMs
	}
	if rwTimeout == 0 {
		rwTimeout = IpcDefaultTimeoutMs
	}
	return &IpcClientPool{
		serverName:  serverName,
		maxConns:    maxConns,
		idleTimeout: idleTimeout,
		connTimeout: connTimeout,
		rwTimeout:   rwTimeout,
		idleConns:   make([]*IpcConnection, 0),
	}
}

func (p *IpcClientPool) acquire() (*IpcConnection, error) {
	p.mu.Lock()
	if len(p.idleConns) > 0 {
		conn := p.idleConns[len(p.idleConns)-1]
		p.idleConns = p.idleConns[:len(p.idleConns)-1]
		p.activeCount++
		p.mu.Unlock()
		if conn.IsValid() {
			return conn, nil
		}
		conn.Close()
		p.mu.Lock()
		p.activeCount--
		p.mu.Unlock()
	}

	if p.activeCount >= p.maxConns {
		p.mu.Unlock()
		return nil, fmt.Errorf("connection pool exhausted: max %d connections", p.maxConns)
	}

	p.activeCount++
	p.mu.Unlock()

	conn := NewIpcConnection()
	if err := conn.Connect(p.serverName, p.connTimeout, p.rwTimeout); err != nil {
		p.mu.Lock()
		p.activeCount--
		p.mu.Unlock()
		return nil, err
	}
	return conn, nil
}

func (p *IpcClientPool) release(conn *IpcConnection, valid bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeCount--

	if valid && conn.IsValid() {
		p.idleConns = append(p.idleConns, conn)
	} else {
		conn.Close()
	}
}

func (p *IpcClientPool) Send(request []byte, callback ClientRecvDataCallback) error {
	conn, err := p.acquire()
	if err != nil {
		return err
	}

	if err := conn.WriteEncrypted(request, p.rwTimeout); err != nil {
		p.release(conn, false)
		return fmt.Errorf("write failed: %w", err)
	}

	wrapperCb := func(data []byte) bool {
		result := callback(data)
		p.release(conn, true)
		return result
	}

	return conn.ReadLoop(wrapperCb, p.rwTimeout)
}

func (p *IpcClientPool) SendAsync(request []byte) error {
	conn, err := p.acquire()
	if err != nil {
		return err
	}

	go func() {
		conn.WriteEncrypted(request, p.rwTimeout)
		p.release(conn, conn.IsValid())
	}()

	return nil
}

func (p *IpcClientPool) CleanupIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	active := make([]*IpcConnection, 0, len(p.idleConns))
	for _, conn := range p.idleConns {
		if conn.IsValid() {
			active = append(active, conn)
		} else {
			conn.Close()
		}
	}
	p.idleConns = active
	_ = now
}
