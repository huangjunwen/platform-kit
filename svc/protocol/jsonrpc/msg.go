package jsonrpc

import (
	"encoding/json"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/opt"
)

type ver20 struct{}

// easyjson:json
type request struct {
	Ver ver20 `json:"jsonrpc"`

	Method string `json:"method"`

	// params 只能是 Object 或者 Array
	Params interface{} `json:"params,omitempty"`

	// id 只能是字符串或者是数字，在这个实现中不支持 notification，id 必填
	ID interface{} `json:"id"`

	// 扩展
	Context map[string]string `json:"ctx,omitempty"`
}

// easyjson:json
type response struct {
	Ver ver20 `json:"jsonrpc"`

	// 出错的时候该字段必须不存在
	Result interface{} `json:"result,omitempty"`

	// 没出错的时候该字段必须不存在
	Error *responseError `json:"error,omitempty"`

	ID interface{} `json:"id"`
}

// easyjson:json
type responseError struct {
	// 这里使用 opt.Int 是因为 Unmarshal 的时候需要区分 0 值和缺少
	Code opt.Int `json:"code"`

	Message string `json:"message"`

	Data interface{} `json:"data,omitempty"`
}

// ResponseError 代表一个 jsonrpc 响应错误，客户端在收到 error 时可以作类型判断，
// 若符合 ResponseError 则可以提取额外信息，具体见：http://www.jsonrpc.org/specification#error_object
type ResponseError interface {
	error

	// ErrCode 返回错误代码
	ErrCode() int

	// ErrMessage 返回错误信息
	ErrMessage() string

	// ErrData 返回额外数据
	ErrData() json.RawMessage
}

func (v ver20) MarshalJSON() ([]byte, error) {
	return []byte(`"2.0"`), nil
}

func (v ver20) UnmarshalJSON(data []byte) error {
	// NOTE: 忽略
	return nil
}

func (e *responseError) Error() string {
	return e.Message
}

func (e *responseError) ErrCode() int {
	return e.Code.V
}

func (e *responseError) ErrMessage() string {
	return e.Message
}

func (e *responseError) ErrData() json.RawMessage {
	ret, ok := e.Data.(*easyjson.RawMessage)
	if !ok {
		return nil
	}
	return json.RawMessage(*ret)
}
