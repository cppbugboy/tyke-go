package common

type Result[T any] struct {
	Value T
	Err   string
}

func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value}
}

func Err[T any](err string) Result[T] {
	return Result[T]{Err: err}
}

func (r Result[T]) HasValue() bool {
	return r.Err == ""
}

func (r Result[T]) HasError() bool {
	return r.Err != ""
}

type BoolResult = Result[bool]

type ByteVecResult = Result[[]byte]

func OkBool(value bool) BoolResult {
	return Ok(value)
}

func ErrBool(err string) BoolResult {
	return Err[bool](err)
}

func OkByteVec(value []byte) ByteVecResult {
	return Ok(value)
}

func ErrByteVec(err string) ByteVecResult {
	return Err[[]byte](err)
}
