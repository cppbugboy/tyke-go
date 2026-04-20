// Package common 定义了 Tyke 框架的公共类型、常量和工具函数。
package common

import "encoding/json"

// JsonValue 定义了 JSON 值的变体类型，支持多种基本数据类型。
type JsonValue any

func VariantToJson(v JsonValue) json.RawMessage {
	switch val := v.(type) {
	case nil:
		return json.RawMessage("null")
	case bool:
		b, _ := json.Marshal(val)
		return b
	case int:
		b, _ := json.Marshal(val)
		return b
	case int64:
		b, _ := json.Marshal(val)
		return b
	case float64:
		b, _ := json.Marshal(val)
		return b
	case string:
		b, _ := json.Marshal(val)
		return b
	default:
		b, _ := json.Marshal(val)
		return b
	}
}

func JsonToVariant(data json.RawMessage) JsonValue {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return string(data)
	}
	switch v := raw.(type) {
	case nil:
		return nil
	case bool:
		return v
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case string:
		return v
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
