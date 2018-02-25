// Package libsvc 提供一个服务框架，该框架的核心类型是“服务” Service ：一组可供调用的方法
//
// 服务可以有很多种实现，例如在本进程内有实际实现的服务：NewLocalService，
// 又如在远程进程中实现的远程服务等，最终使用者面对的都是统一的 Service 接口
package libsvc
