// Package component 提供tyke框架的核心可复用组件。
//
// 本文件实现了一个泛型线程安全的单例模式。
// 单例模式确保一个类型只有一个实例，并在整个应用程序中共享。
//
// # 主要特性
//
// - 泛型实现，支持任意类型
// - 使用sync.Once保证线程安全的初始化
// - 首次访问时延迟初始化
//
// # 使用示例
//
//	var dbSingleton component.Singleton[*Database]
//
//	func GetDatabase() *Database {
//	    return dbSingleton.GetInstance(func() *Database {
//	        return NewDatabase("connection-string")
//	    })
//	}
//
// # 作者
//
// Nick
package component

import "sync"

// Singleton 是一个延迟初始化的单例实例容器。
//
// 它使用sync.Once确保实例只创建一次，
// 即使多个goroutine同时尝试访问也是如此。
//
// 类型参数T可以是任意类型。单例存储指向T的指针。
//
// 示例：
//
//	var cacheSingleton Singleton[*Cache]
//	cache := cacheSingleton.GetInstance(func() *Cache {
//	    return NewCache(1000)
//	})
type Singleton[T any] struct {
	instance *T        // 单例实例
	once     sync.Once // 确保单次初始化
}

// GetInstance 返回单例实例，必要时创建它。
//
// 如果实例尚未创建，则调用creator函数来创建它。
// 后续调用返回相同的实例，而不会再次调用creator。
//
// 参数：
//   - creator: 创建T类型新实例的函数。
//     仅在首次访问时调用一次。
//
// 返回：
//   - *T: 单例实例（如果creator返回nil，可能为nil）。
//
// 线程安全：此方法可安全用于并发。
// creator函数最多调用一次。
//
// 示例：
//
//	singleton := Singleton[*Config]{}
//	cfg := singleton.GetInstance(func() *Config {
//	    return LoadConfig("config.yaml")
//	})
func (s *Singleton[T]) GetInstance(creator func() *T) *T {
	s.once.Do(func() {
		s.instance = creator()
	})
	return s.instance
}
