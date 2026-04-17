// Copyright 2026 Tyke Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

// NewBoolResult 创建一个布尔类型的结果。
//
// 在Go中，布尔结果通常用error表示：
//   - nil 表示成功
//   - 非 nil 表示失败，错误信息包含在error中
//
// 参数:
//   - err: 错误对象，nil表示成功
//
// 返回值:
//   - error: 传入的error对象，直接返回
//
// 示例:
//
//	// 成功情况
//	result := NewBoolResult(nil)
//
//	// 失败情况
//	result := NewBoolResult(fmt.Errorf("operation failed"))
func NewBoolResult(err error) error {
	return err
}

// NewByteVecResult 创建一个字节切片类型的结果。
//
// 这是Go中最常用的结果返回模式，返回值和错误分开：
//   - data: 成功时返回的数据
//   - err: 错误信息，nil表示成功
//
// 参数:
//   - data: 成功时返回的字节数据
//   - err: 错误对象，nil表示成功
//
// 返回值:
//   - []byte: 传入的字节数据
//   - error: 传入的error对象
//
// 示例:
//
//	// 成功情况
//	data, err := NewByteVecResult([]byte("hello"), nil)
//
//	// 失败情况
//	data, err := NewByteVecResult(nil, fmt.Errorf("read failed"))
func NewByteVecResult(data []byte, err error) ([]byte, error) {
	return data, err
}
