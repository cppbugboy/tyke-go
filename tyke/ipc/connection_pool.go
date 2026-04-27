package ipc

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"tyke-go/common"
)

type ConnectionPoolConfig struct {
	MaxConnections     int
	MinIdleConnections int
	IdleTimeoutMs      uint32
	ConnectTimeoutMs   uint32
	RwTimeoutMs        uint32
	AcquireTimeoutMs   uint32
}

func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxConnections:     IPCDefaultMaxConnections,
		MinIdleConnections: 1,
		IdleTimeoutMs:      IPCDefaultIdleTimeoutMs,
		ConnectTimeoutMs:   IPCDefaultTimeoutMs,
		RwTimeoutMs:        IPCDefaultTimeoutMs,
		AcquireTimeoutMs:   3000,
	}
}

type ConnectionPool struct {
	serverUuid       string
	config           ConnectionPoolConfig
	idle             []*IPCConnection
	active           int32
	mu               sync.Mutex
	available        chan struct{}
	stopCh           chan struct{}
	wg               sync.WaitGroup
	createConnection func() *IPCConnection
	stopped          atomic.Bool
}

func NewConnectionPool(serverUuid string, config ConnectionPoolConfig) *ConnectionPool {
	p := &ConnectionPool{
		serverUuid: serverUuid,
		config:     config,
		stopCh:     make(chan struct{}),
		available:  make(chan struct{}, 1),
	}
	p.createConnection = p.defaultCreateConnection

	p.wg.Add(1)
	go p.cleanupLoop()

	common.LogInfo("Connection pool created", "server_uuid", serverUuid, "max", config.MaxConnections, "min_idle", config.MinIdleConnections)
	return p
}

func (p *ConnectionPool) Acquire() (*IPCConnection, error) {
	p.mu.Lock()

	if p.stopped.Load() {
		p.mu.Unlock()
		return nil, fmt.Errorf("connection pool is stopped, server=%s", p.serverUuid)
	}

	for len(p.idle) > 0 {
		conn := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]

		if conn.IsValid() {
			atomic.AddInt32(&p.active, 1)
			conn.UpdateLastUsedTime()
			p.mu.Unlock()
			common.LogDebug("Acquired idle connection from pool", "server", p.serverUuid,
				"idle", len(p.idle), "active", atomic.LoadInt32(&p.active))
			return conn, nil
		}
		common.LogWarn("Idle connection invalid, destroying", "server", p.serverUuid)
		conn.Close()
	}

	canCreate := int(atomic.LoadInt32(&p.active)) < p.config.MaxConnections
	if canCreate {
		atomic.AddInt32(&p.active, 1)
	}
	p.mu.Unlock()

	if canCreate {
		conn := p.createConnection()
		if conn != nil {
			common.LogDebug("Created new connection in pool", "server", p.serverUuid,
				"idle", len(p.idle), "active", atomic.LoadInt32(&p.active))
			return conn, nil
		}
		atomic.AddInt32(&p.active, -1)
		return nil, fmt.Errorf("failed to create connection for pool, server=%s", p.serverUuid)
	}

	timer := time.NewTimer(time.Duration(p.config.AcquireTimeoutMs) * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-p.available:
			if p.stopped.Load() {
				return nil, fmt.Errorf("connection pool stopped, server=%s", p.serverUuid)
			}
		case <-timer.C:
			return nil, fmt.Errorf("acquire connection timeout, server=%s", p.serverUuid)
		case <-p.stopCh:
			return nil, fmt.Errorf("connection pool stopped, server=%s", p.serverUuid)
		}

		p.mu.Lock()
		if p.stopped.Load() {
			p.mu.Unlock()
			return nil, fmt.Errorf("connection pool stopped, server=%s", p.serverUuid)
		}

		for len(p.idle) > 0 {
			conn := p.idle[len(p.idle)-1]
			p.idle = p.idle[:len(p.idle)-1]

			if conn.IsValid() {
				atomic.AddInt32(&p.active, 1)
				conn.UpdateLastUsedTime()
				p.mu.Unlock()
				return conn, nil
			}
			common.LogWarn("Idle connection invalid, destroying", "server", p.serverUuid)
			conn.Close()
		}

		if int(atomic.LoadInt32(&p.active)) < p.config.MaxConnections {
			atomic.AddInt32(&p.active, 1)
			p.mu.Unlock()
			conn := p.createConnection()
			if conn != nil {
				return conn, nil
			}
			atomic.AddInt32(&p.active, -1)
			return nil, fmt.Errorf("failed to create connection for pool, server=%s", p.serverUuid)
		}

		p.mu.Unlock()
	}
}

func (p *ConnectionPool) Release(conn *IPCConnection, shouldReconnect bool) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped.Load() {
		conn.Close()
		atomic.AddInt32(&p.active, -1)
		return
	}

	if shouldReconnect || !conn.IsValid() {
		common.LogWarn("Releasing broken connection, reconnecting", "server", p.serverUuid)
		conn.Close()

		total := len(p.idle) + int(atomic.LoadInt32(&p.active))
		if total-1 < p.config.MinIdleConnections {
			p.mu.Unlock()
			if newConn := p.createConnection(); newConn != nil {
				p.mu.Lock()
				p.idle = append(p.idle, newConn)
				common.LogDebug("Created replacement connection", "server", p.serverUuid)
			} else {
				p.mu.Lock()
			}
		}
	} else {
		conn.UpdateLastUsedTime()
		p.idle = append(p.idle, conn)
		common.LogDebug("Released connection to pool", "server", p.serverUuid,
			"idle", len(p.idle), "active", atomic.LoadInt32(&p.active)-1)
	}

	atomic.AddInt32(&p.active, -1)
	select {
	case p.available <- struct{}{}:
	default:
	}
}

func (p *ConnectionPool) GetIdleCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idle)
}

func (p *ConnectionPool) GetActiveCount() int {
	return int(atomic.LoadInt32(&p.active))
}

func (p *ConnectionPool) GetServerUuid() string {
	return p.serverUuid
}

func (p *ConnectionPool) Stop() {
	if !p.stopped.CompareAndSwap(false, true) {
		return
	}

	close(p.stopCh)
	select {
	case p.available <- struct{}{}:
	default:
	}
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.idle {
		conn.Close()
	}
	p.idle = nil
	atomic.StoreInt32(&p.active, 0)

	common.LogInfo("Connection pool stopped", "server_uuid", p.serverUuid)
}

func (p *ConnectionPool) defaultCreateConnection() *IPCConnection {
	conn := NewIPCConnection()
	result := conn.Connect(p.serverUuid, p.config.ConnectTimeoutMs)
	if !result.HasValue() {
		common.LogError("Failed to connect new connection", "server", p.serverUuid, "error", result.Err)
		conn.Close()
		return nil
	}
	common.LogDebug("Created and connected new connection", "server", p.serverUuid)
	return conn
}

func (p *ConnectionPool) cleanupLoop() {
	defer p.wg.Done()

	interval := time.Duration(p.config.IdleTimeoutMs) * time.Millisecond / 2
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.cleanupIdleConnections()
		}
	}
}

func (p *ConnectionPool) cleanupIdleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	timeout := time.Duration(p.config.IdleTimeoutMs) * time.Millisecond

	i := 0
	for i < len(p.idle) {
		conn := p.idle[i]
		shouldRemove := false

		elapsed := now.Sub(conn.GetLastUsedTime())
		if elapsed > timeout {
			shouldRemove = true
			common.LogDebug("Idle connection timeout, removing", "server", p.serverUuid)
		}

		if !conn.IsValid() {
			shouldRemove = true
			common.LogDebug("Idle connection invalid, removing", "server", p.serverUuid)
		}

		remaining := len(p.idle) + int(atomic.LoadInt32(&p.active))
		if shouldRemove && remaining <= p.config.MinIdleConnections {
			i++
			continue
		}

		if shouldRemove {
			conn.Close()
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
		} else {
			i++
		}
	}
}
