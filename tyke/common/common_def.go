// Package common 为 Tyke 框架提供共享类型定义。
//
// 本文件定义了 JsonValueHolder 类型，用于在元数据键值对中
// 表示 JSON 原始值，以及相关的转换辅助函数。
package common

import (
	"encoding/json"
	"fmt"
)

// JsonValue 是有效 JSON 原始值类型的类型约束。
type JsonValue interface {
	bool | int | int64 | float64 | string
}

// JsonValueHolder 包装一个 JSON 原始值，用于元数据映射中。
// 支持 bool、int、int64、float64、string 和 nil。
type JsonValueHolder struct {
	value any
}

// NewJsonValue 从 JSON 原始值创建一个 JsonValueHolder。
// 对于 nil 值请使用 NewJsonNilValue，因为 nil 不在 JsonValue 约束中。
func NewJsonValue[T JsonValue](v T) JsonValueHolder {
	return JsonValueHolder{value: v}
}

// NewJsonNilValue 创建一个包含 nil 值的 JsonValueHolder。
func NewJsonNilValue() JsonValueHolder {
	return JsonValueHolder{value: nil}
}

// Value 返回 JsonValueHolder 持有的底层值。
func (j JsonValueHolder) Value() any {
	return j.value
}

// VariantToJson 将 JsonValueHolder 转换为其 JSON 表示形式。
func VariantToJson(v JsonValueHolder) json.RawMessage {
	switch val := v.value.(type) {
	case nil:
		return json.RawMessage("null")
	case bool:
		b, err := json.Marshal(val)
		if err != nil {
			return json.RawMessage("null")
		}
		return b
	case int:
		b, err := json.Marshal(val)
		if err != nil {
			return json.RawMessage("null")
		}
		return b
	case int64:
		b, err := json.Marshal(val)
		if err != nil {
			return json.RawMessage("null")
		}
		return b
	case float64:
		b, err := json.Marshal(val)
		if err != nil {
			return json.RawMessage("null")
		}
		return b
	case string:
		b, err := json.Marshal(val)
		if err != nil {
			return json.RawMessage("null")
		}
		return b
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return json.RawMessage("null")
		}
		return b
	}
}

// JsonToVariant 将原始 JSON 消息转换为 JsonValueHolder。
// 优先尝试反序列化为原生 Go 类型（bool、float64、string），
// 对于无法识别的值则回退到字符串表示形式。
func JsonToVariant(data json.RawMessage) JsonValueHolder {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return NewJsonValue(string(data))
	}
	switch v := raw.(type) {
	case nil:
		return NewJsonNilValue()
	case bool:
		return NewJsonValue(v)
	case float64:
		if v == float64(int64(v)) {
			return NewJsonValue(int64(v))
		}
		return NewJsonValue(v)
	case string:
		return NewJsonValue(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return NewJsonValue(fmt.Sprintf("%v", v))
		}
		return NewJsonValue(string(b))
	}
}
