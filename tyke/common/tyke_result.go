// Package common 提供共享类型定义。
//
// 本文件定义了泛型 Result[T] 类型，这是一种 Rust 风格的结果类型，
// 表示一个成功的值或一个错误字符串。
package common

// Result 表示一个操作结果，可以是成功值或错误。
// ok 字段跟踪结果是否为成功。
type Result[T any] struct {
	Value T
	Err   string
	ok    bool
}

// Ok 创建一个包含给定值的成功 Result。
func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value, ok: true}
}

// Err 创建一个包含给定错误消息的失败 Result。
func Err[T any](err string) Result[T] {
	return Result[T]{Err: err}
}

// HasValue 如果 Result 表示一个成功值则返回 true。
func (r Result[T]) HasValue() bool {
	return r.ok
}

// HasError 如果 Result 表示一个错误则返回 true。
func (r Result[T]) HasError() bool {
	return !r.ok
}

// Error 实现 error 接口，使 Result 能够作为标准错误使用。
func (r Result[T]) Error() string {
	return r.Err
}

// BoolResult 是用于布尔操作的 Result 别名。
type BoolResult = Result[bool]

// ByteVecResult 是用于字节切片操作的 Result 别名。
type ByteVecResult = Result[[]byte]

// OkBool 创建一个成功的 BoolResult。
func OkBool(value bool) BoolResult {
	return Ok(value)
}

// ErrBool 创建一个失败的 BoolResult。
func ErrBool(err string) BoolResult {
	return Err[bool](err)
}

// OkByteVec 创建一个成功的 ByteVecResult。
func OkByteVec(value []byte) ByteVecResult {
	return Ok(value)
}

// ErrByteVec 创建一个失败的 ByteVecResult。
func ErrByteVec(err string) ByteVecResult {
	return Err[[]byte](err)
}
