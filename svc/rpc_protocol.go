package libsvc

import (
	"io"
)

// RPCServerProtocolFactory 代表 RPC 服务端协议工厂，主要负责传输层面数据与方法层面对象之间的转换；
// 每当接收到 RPC 请求时，应当调用 Protocol 方法获得一个新的协议对象来处理
type RPCServerProtocolFactory interface {
	Protocol() RPCServerProtocol
}

// RPCClientProtocolFactory 代表 RPC 客户端协议工厂，主要负责传输层面数据与方法层面对象之间的转换；
// 每当发起 RPC 请求时，应当调用 Protocol 方法获得一个新的协议对象来处理
type RPCClientProtocolFactory interface {
	Protocol() RPCClientProtocol
}

// RPCServerProtocol 代表 RPC 服务端协议
//
// NOTE: 以下各步骤有共通点：
//   1. err 为服务端内部错误，由服务端自行处理（例如记录日志），不会返回给客户端，
//      业务错误或是客户端错误应该在响应中体现而非此处；当 err 非 nil 时流程马上结束
//   2. 当 err 为 nil 时，若 done 为 true 则流程也马上结束
//   3. respWriter 用于输出响应数据，若其 Write 方法返回错误，应当作为服务端内部错误返回
type RPCServerProtocol interface {
	// ProcessRequest 在接收到请求时触发，RPCServerProtocol 可以在这里解析请求数据；
	// 正常情况下应当解析出所请求的 methodName 以及携带的 passthru 数据；
	ProcessRequest(respWriter io.Writer, reqReader io.Reader) (done bool, methodName string, passthru map[string]string, err error)

	// ProcessMethodNotFound 在 ProcessRequest 成功解析出方法名称后且服务找不到该方法时触发；
	// 该步骤完成后流程结束
	ProcessMethodNotFound(respWriter io.Writer, methodName string) (err error)

	// ProcessInput 在服务成功找到方法时触发，RPCServerProtocol 应该在此处理入参：
	// 例如从请求数据中解析出入参的数据填入 input 中；
	ProcessInput(respWriter io.Writer, input interface{}) (done bool, err error)

	// ProcessOutput 在方法处理完时触发，RPCServerProtocol 应该在此处理实际业务逻辑产生
	// 的出参以及错误：该步骤完成后流程结束
	ProcessOutput(respWriter io.Writer, output interface{}, outputErr error) (err error)
}

// RPCClientProtocol 代表客户端协议
type RPCClientProtocol interface {
	// ProcessInput 在开始 rpc 请求时触发，RPCClientProtocol 应当序列化请求，
	// 或是返回错误以终止下面的步骤；passthru 如果有数据，协议应当将之原封不动地传到服务端
	ProcessInput(reqWriter io.Writer, methodName string, input interface{}, passthru map[string]string) error

	// ProcessOutput 在有响应返回时触发，RPCClientProtocol 应当反序列化响应到 output 中或是
	// 返回错误
	ProcessOutput(respReader io.Reader, output interface{}) error
}
