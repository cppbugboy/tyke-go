package ipc

import (
	"sync"

	"github.com/tyke/tyke/tyke/common"
)

type ConnectionPoolFactory struct {
	pools sync.Map
	mu    sync.Mutex
}

var (
	factoryInstance *ConnectionPoolFactory
	factoryOnce     sync.Once
)

func GetConnectionPoolFactory() *ConnectionPoolFactory {
	factoryOnce.Do(func() {
		factoryInstance = &ConnectionPoolFactory{}
		common.LogInfo("ConnectionPoolFactory initialized")
	})
	return factoryInstance
}

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

func (f *ConnectionPoolFactory) RemovePool(serverUuid string) {
	if val, loaded := f.pools.LoadAndDelete(serverUuid); loaded {
		pool := val.(*ConnectionPool)
		pool.Stop()
		common.LogInfo("Removed connection pool", "server_uuid", serverUuid)
	}
}

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
