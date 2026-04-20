package ipc

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tyke/tyke/tyke/common"
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
		MaxConnections:     IpcDefaultMaxConnections,
		MinIdleConnections: 1,
		IdleTimeoutMs:      IpcDefaultIdleTimeoutMs,
		ConnectTimeoutMs:   IpcDefaultTimeoutMs,
		RwTimeoutMs:        IpcDefaultTimeoutMs,
		AcquireTimeoutMs:   3000,
	}
}

type ConnectionPool struct {
	serverUuid       string
	config           ConnectionPoolConfig
	idle             []*IpcConnection
	active           int32
	mu               sync.Mutex
	cond             *sync.Cond
	stopCh           chan struct{}
	wg               sync.WaitGroup
	createConnection func() *IpcConnection
}

func NewConnectionPool(serverUuid string, config ConnectionPoolConfig) *ConnectionPool {
	p := &ConnectionPool{
		serverUuid: serverUuid,
		config:     config,
		stopCh:     make(chan struct{}),
	}
	p.createConnection = p.defaultCreateConnection
	p.cond = sync.NewCond(&p.mu)

	p.wg.Add(1)
	go p.cleanupLoop()

	common.LogInfo("Connection pool created", "server_uuid", serverUuid, "max", config.MaxConnections, "min_idle", config.MinIdleConnections)
	return p
}

func (p *ConnectionPool) Acquire() (*IpcConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		for len(p.idle) > 0 {
			conn := p.idle[len(p.idle)-1]
			p.idle = p.idle[:len(p.idle)-1]

			if conn.IsValid() {
				atomic.AddInt32(&p.active, 1)
				conn.UpdateLastUsedTime()
				common.LogDebug("Acquired idle connection from pool", "server", p.serverUuid,
					"idle", len(p.idle), "active", atomic.LoadInt32(&p.active))
				return conn, nil
			}
			common.LogWarn("Idle connection invalid, destroying", "server", p.serverUuid)
			conn.Close()
		}

		if int(atomic.LoadInt32(&p.active)) < p.config.MaxConnections {
			conn := p.createConnection()
			if conn != nil {
				atomic.AddInt32(&p.active, 1)
				common.LogDebug("Created new connection in pool", "server", p.serverUuid,
					"idle", len(p.idle), "active", atomic.LoadInt32(&p.active))
				return conn, nil
			}
			return nil, fmt.Errorf("failed to create connection for pool, server=%s", p.serverUuid)
		}

		deadline := time.Now().Add(time.Duration(p.config.AcquireTimeoutMs) * time.Millisecond)

		timer := time.NewTimer(time.Duration(p.config.AcquireTimeoutMs) * time.Millisecond)
		waitCh := make(chan struct{})
		go func() {
			<-timer.C
			p.cond.Broadcast()
			close(waitCh)
		}()

		p.cond.Wait()

		timedOut := false
		select {
		case <-waitCh:
			timedOut = true
		default:
		}
		timer.Stop()

		if timedOut || time.Now().After(deadline) {
			return nil, fmt.Errorf("acquire connection timeout, server=%s", p.serverUuid)
		}
	}
}

func (p *ConnectionPool) Release(conn *IpcConnection, shouldReconnect bool) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if shouldReconnect || !conn.IsValid() {
		common.LogWarn("Releasing broken connection, reconnecting", "server", p.serverUuid)
		conn.Close()

		total := len(p.idle) + int(atomic.LoadInt32(&p.active))
		if total-1 < p.config.MinIdleConnections {
			if newConn := p.createConnection(); newConn != nil {
				p.idle = append(p.idle, newConn)
				common.LogDebug("Created replacement connection", "server", p.serverUuid)
			}
		}
	} else {
		conn.UpdateLastUsedTime()
		p.idle = append(p.idle, conn)
		common.LogDebug("Released connection to pool", "server", p.serverUuid,
			"idle", len(p.idle), "active", atomic.LoadInt32(&p.active)-1)
	}

	atomic.AddInt32(&p.active, -1)
	p.cond.Signal()
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
	select {
	case <-p.stopCh:
		return
	default:
	}

	close(p.stopCh)
	p.cond.Broadcast()
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

func (p *ConnectionPool) defaultCreateConnection() *IpcConnection {
	conn := NewIpcConnection()
	result := conn.Connect(p.serverUuid, p.config.ConnectTimeoutMs, p.config.RwTimeoutMs)
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

	interval := time.Duration(p.config.IdleTimeoutMs/2) * time.Millisecond
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
