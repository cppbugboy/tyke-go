package common

import (
	"encoding/json"
	"fmt"
)

type JsonValue interface {
	bool | int | int64 | float64 | string
}

type JsonValueHolder struct {
	value any
}

func NewJsonValue(v any) JsonValueHolder {
	return JsonValueHolder{value: v}
}

func (j JsonValueHolder) Value() any {
	return j.value
}

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

func JsonToVariant(data json.RawMessage) JsonValueHolder {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return NewJsonValue(string(data))
	}
	switch v := raw.(type) {
	case nil:
		return NewJsonValue(nil)
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
