package common

type Result[T any] struct {
	Value T
	Err   string
	ok    bool
}

func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value, ok: true}
}

func Err[T any](err string) Result[T] {
	return Result[T]{Err: err}
}

func (r Result[T]) HasValue() bool {
	return r.ok
}

func (r Result[T]) HasError() bool {
	return !r.ok
}

// Error 实现 error 接口，允许 Result 作为标准 error 使用。
func (r Result[T]) Error() string {
	return r.Err
}

// BoolResult 表示布尔类型的操作结果，包含成功值或错误信息。
type BoolResult = Result[bool]

// ByteVecResult 是字节切片类型的结果别名。
type ByteVecResult = Result[[]byte]

// OkBool 创建一个包含成功值的 BoolResult。
func OkBool(value bool) BoolResult {
	return Ok(value)
}

// ErrBool 创建一个包含错误信息的 BoolResult。
func ErrBool(err string) BoolResult {
	return Err[bool](err)
}

func OkByteVec(value []byte) ByteVecResult {
	return Ok(value)
}

// ErrByteVec 创建一个包含错误信息的 ByteVecResult。
func ErrByteVec(err string) ByteVecResult {
	return Err[[]byte](err)
}
