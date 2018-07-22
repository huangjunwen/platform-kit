package jsonrpc

import (
	"encoding/json"
	"errors"
	"io"

	libsvc "github.com/huangjunwen/platform-kit/svc"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/opt"
	"github.com/rs/xid"
)

const (
	codeParseError     = -32700
	codeInvalidReq     = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	codeGeneralError   = -1
)

const (
	msgParseError     = "Parse error"
	msgInvalidReq     = "Invalid request"
	msgMethodNotFound = "Method not found"
	msgInvalidParams  = "Invalid params"
	msgInternalError  = "Internal error"
	msgGeneralError   = "General error"
)

var (
	missingID     = easyjson.RawMessage(`"Missing field 'id'"`)
	badIDValue    = easyjson.RawMessage(`"Field 'id' should be string or number"`)
	badParamValue = easyjson.RawMessage(`"Field 'param' should be object or array"`)
	missingMethod = easyjson.RawMessage(`"Missing field 'method'"`)
)

var (
	errIDMismatch = errors.New("Request/response id mismatch")
)

var (
	// ServerProtocolFactory 为 jsonrpc 服务端协议工厂
	ServerProtocolFactory libsvc.RPCServerProtocolFactory = serverProtocolFactory{}
	// ClientProtocolFactory 为 jsonrpc 客户端协议工厂
	ClientProtocolFactory libsvc.RPCClientProtocolFactory = clientProtocolFactory{}
)

type serverProtocolFactory struct{}

type clientProtocolFactory struct{}

type serverProtocol struct {
	// params 延迟解析
	params easyjson.RawMessage
	// 请求的 id，不需要解析，只需要检查类型，响应时原样返回
	id easyjson.RawMessage
}

type clientProtocol struct {
	// 记录下请求的 id，用于对比响应
	id string
}

func (f serverProtocolFactory) Protocol() libsvc.RPCServerProtocol {
	return &serverProtocol{}
}

func (f clientProtocolFactory) Protocol() libsvc.RPCClientProtocol {
	return &clientProtocol{}
}

func (p *serverProtocol) writeErrorResponse(respWriter io.Writer, code int, message string, data interface{}) error {
	resp := response{
		Error: &responseError{
			Code:    opt.OInt(code),
			Message: message,
			Data:    data,
		},
	}
	if len(p.id) != 0 {
		resp.ID = &p.id
	}
	if _, err := easyjson.MarshalToWriter(resp, respWriter); err != nil {
		return err
	}
	return nil
}

func (p *serverProtocol) writeResponse(respWriter io.Writer, result interface{}) error {
	resp := response{
		Result: result,
	}
	if len(p.id) != 0 {
		resp.ID = &p.id
	}
	if _, err := easyjson.MarshalToWriter(resp, respWriter); err != nil {
		return err
	}
	return nil
}

func (p *serverProtocol) ProcessRequest(respWriter io.Writer, reqReader io.Reader) (done bool, methodName string, passthru map[string]string, err error) {
	// Unmarshal 时是一个 *easyjson.RawMessage 以延迟求值，该技巧见: http://eagain.net/articles/go-dynamic-json/
	id := easyjson.RawMessage{}
	params := easyjson.RawMessage{}
	req := request{
		ID:     &id,
		Params: &params,
	}

	// 从 req 解析出来，若有错误返回 Parse error
	if err := unmarshalFromReader(reqReader, &req); err != nil {
		return true, "", nil, p.writeErrorResponse(respWriter, codeParseError, msgParseError, nil)
	}

	// 检查 ID
	if len(id) == 0 {
		// 缺 ID
		return true, "", nil, p.writeErrorResponse(respWriter, codeInvalidReq, msgInvalidReq, missingID)
	}
	switch id[0] {
	// ID 应该是字符串或者是数字，json 格式没问题，所以只需要检查第一个字符即可
	case '"', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
	default:
		return true, "", nil, p.writeErrorResponse(respWriter, codeInvalidReq, msgInvalidReq, badIDValue)
	}
	p.id = id

	// 检查 Params
	if len(params) != 0 {
		// Params 应该是 array 或者 object，json 格式没问题，所以只需要检查第一个字符即可
		switch params[0] {
		case '{', '[':
		default:
			return true, "", nil, p.writeErrorResponse(respWriter, codeInvalidReq, msgInvalidReq, badParamValue)
		}
	}
	p.params = params

	// 检查方法
	if req.Method == "" {
		return true, "", nil, p.writeErrorResponse(respWriter, codeInvalidReq, msgInvalidReq, missingMethod)
	}

	return false, req.Method, req.Context, nil

}

func (p *serverProtocol) ProcessMethodNotFound(respWriter io.Writer, methodName string) error {
	return p.writeErrorResponse(respWriter, codeMethodNotFound, msgMethodNotFound, methodName)
}

func (p *serverProtocol) ProcessInput(respWriter io.Writer, input interface{}) (done bool, err error) {
	// 没有，跳过这步
	if len(p.params) == 0 {
		return false, nil
	}
	// 解析入参
	switch i := input.(type) {
	case easyjson.Unmarshaler:
		err = easyjson.Unmarshal(p.params, i)
	default:
		err = json.Unmarshal(p.params, i)
	}
	if err != nil {
		return true, p.writeErrorResponse(respWriter, codeInvalidParams, msgInvalidParams, err.Error())
	}
	return false, nil

}

func (p *serverProtocol) ProcessOutput(respWriter io.Writer, output interface{}, outputErr error) error {
	if outputErr == nil {
		// 没有错误
		return p.writeResponse(respWriter, output)
	}
	// 若返回的 outputErr 可以被 json 序列化，则序列化之，否则序列化其 Error 字符串
	data := interface{}(nil)
	switch outputErr.(type) {
	case json.Marshaler, easyjson.Marshaler:
		data = outputErr
	default:
		data = outputErr.Error()
	}
	return p.writeErrorResponse(respWriter, codeGeneralError, msgGeneralError, data)
}

func (p *clientProtocol) ProcessInput(reqWriter io.Writer, methodName string, input interface{}, passthru map[string]string) error {
	// NOTE: char set is 0-9, a-v，因此 json 序列化/反序列化时是不需要 escape 的
	id := xid.New().String()

	// 装配 request 对象
	req := request{
		Method: methodName,
		ID:     id,
	}
	if input != nil {
		req.Params = input
	}
	if len(passthru) != 0 {
		req.Context = passthru
	}

	// 序列化请求
	if _, err := easyjson.MarshalToWriter(req, reqWriter); err != nil {
		return err
	}

	// 记录下来
	p.id = id
	return nil
}

func (p *clientProtocol) ProcessOutput(respReader io.Reader, output interface{}) error {
	// 反序列化响应
	id := easyjson.RawMessage{}
	resp := response{
		// XXX: 保证 Result 满足 json.Unmarshaler 接口，这样 UnmarshalFromReader
		// 才不会擅自把 Result 改成 map[string]interface{}
		Result: ensureUnmarshaler(output),
		Error: &responseError{
			Data: &easyjson.RawMessage{},
		},
		ID: &id,
	}
	if err := unmarshalFromReader(respReader, &resp); err != nil {
		return err
	}

	// 判断是否有错误响应，有错误响应时无视 id 检查吧
	if resp.Error.Code.IsDefined() {
		return resp.Error
	}

	// 判断 id
	if len(id) == 0 || id[0] != '"' {
		return errIDMismatch
	}
	// NOTE: char set is 0-9, a-v，因此 json 序列化/反序列化时是不需要 escape 的
	id = id[1 : len(id)-1]
	// 从 GO 1.5 后 string(id) != p.id 是不会 copy 的，见
	//   https://stackoverflow.com/a/35342389/157235
	//   https://github.com/golang/go/commit/69cd91a5981c49eaaa59b33196bdb5586c18d289
	if string(id) != p.id {
		return errIDMismatch
	}
	return nil

}
