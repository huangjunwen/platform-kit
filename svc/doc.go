/*
Package libsvc 提供一个服务框架，主要类型跟函数之间的关系图如下：

	                                                                        +-- InprocClient()
	                      Make()                                            |
	     +------------------------------------------------ ServiceClient <--+-- NewRPCClient(RPCClientProtocolFactory, RPCTransportClient)
	     |                                                     * ^
	     v      BindInterface()                                * *
	  Service -------------------> ServiceWithInterface  (req) * * (resp)
	                                     ^  |                  * *
	                                     |  |                  * *          +-- InprocServer()
	                                     |  |                  v *          |
	  NewLocalService() -----------------+  +------------> ServiceServer <--+-- NewRPCServer(RPCServerProtocolFactory, RPCTransportServer)
	                                           Register()

核心类型是“服务” Service（以及 ServiceWithInterface）：可对其方法发起调用
*/
package libsvc
