// Package ipc provides an inter-process communication layer.
//
// This file defines the ConnectionPoolFactory singleton, which manages per-server-UUID
// ConnectionPool instances. It provides lazy pool creation and graceful shutdown.
package ipc

import (
	"sync"

	"tyke-go/common"
)

// ConnectionPoolFactory manages a mapping from server UUIDs to ConnectionPool instances.
// It is a singleton accessed via GetConnectionPoolFactory().
type ConnectionPoolFactory struct {
	pools sync.Map
	mu    sync.Mutex
}

var (
	factoryInstance *ConnectionPoolFactory
	factoryOnce     sync.Once
)

// GetConnectionPoolFactory returns the singleton ConnectionPoolFactory instance.
func GetConnectionPoolFactory() *ConnectionPoolFactory {
	factoryOnce.Do(func() {
		factoryInstance = &ConnectionPoolFactory{}
		common.LogInfo("ConnectionPoolFactory initialized")
	})
	return factoryInstance
}

// GetPool returns the ConnectionPool for the given server UUID, creating one if it
// does not exist. An optional config can be passed for the first-time creation.
func (f *ConnectionPoolFactory) GetPool(serverUuid string, config ...ConnectionPoolConfig) *ConnectionPool {
	if val, ok := f.pools.Load(serverUuid); ok {
		return val.(*ConnectionPool)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if val, ok := f.pools.Load(serverUuid); ok {
		return val.(*ConnectionPool)
	}

	cfg := DefaultConnectionPoolConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	pool := NewConnectionPool(serverUuid, cfg)
	f.pools.Store(serverUuid, pool)
	common.LogInfo("Created new connection pool", "server_uuid", serverUuid)
	return pool
}

// RemovePool stops and removes the ConnectionPool for the given server UUID.
func (f *ConnectionPoolFactory) RemovePool(serverUuid string) {
	if val, loaded := f.pools.LoadAndDelete(serverUuid); loaded {
		pool := val.(*ConnectionPool)
		pool.Stop()
		common.LogInfo("Removed connection pool", "server_uuid", serverUuid)
	}
}

// Shutdown stops all managed ConnectionPool instances and clears the factory.
func (f *ConnectionPoolFactory) Shutdown() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pools.Range(func(key, value interface{}) bool {
		pool := value.(*ConnectionPool)
		pool.Stop()
		common.LogInfo("Stopped connection pool", "server_uuid", key.(string))
		return true
	})
	f.pools = sync.Map{}
}
