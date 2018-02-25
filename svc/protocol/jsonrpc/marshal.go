package jsonrpc

import (
	"bytes"
	"encoding/json"
	"github.com/mailru/easyjson"
	"io"
)

// unmarshaler 包装任意对象以使得它满足 json.Unmarshal 接口
type unmarshaler struct {
	obj interface{}
}

var (
	_ json.Unmarshaler = unmarshaler{nil}
)

func (u unmarshaler) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, u.obj)
}

// ensureUnmarshaler 给 interface{} 套上 json.Unmarshaler 接口
func ensureUnmarshaler(obj interface{}) json.Unmarshaler {
	if u, ok := obj.(json.Unmarshaler); ok {
		return u
	}
	return unmarshaler{obj: obj}
}

// unmarshalFromReader 判断 reader 是否是 bytes.Buffer, 如果是的话，
// 直接取出其 byte slice 用于 Unmarshal，这样能避免 UnmarshalFromReader
// 再读一遍
func unmarshalFromReader(reader io.Reader, v easyjson.Unmarshaler) error {
	switch r := reader.(type) {
	case *bytes.Buffer:
		return easyjson.Unmarshal(r.Bytes(), v)
	default:
		return easyjson.UnmarshalFromReader(reader, v)
	}
}
