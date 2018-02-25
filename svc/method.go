package libsvc

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
)

// Method 定义一个方法（单入参单出参），它包含一个方法的元信息：名字以及出入参的类型信息
type Method interface {
	// Name 返回方法的名字
	Name() string

	// GenInput 生成一个空的入参，一般用于序列化/反序列化，所以一般来说是某种结构体的指针，
	// 另外也可以使用 reflect.TypeOf(Method.GenInput()) 获得入参的类型信息
	// NOTE: 生成的入参应当已经满足 AssertInputType 的测试
	GenInput() interface{}

	// AssertInputType 应当对一个入参进行类型检查，若不通过应当 panic
	AssertInputType(input interface{})

	// GenOutput 生成一个空的出参，一般用于序列化/反序列化，所以一般来说是某种结构体的指针，
	// 另外也可以使用 reflect.TypeOf(Method.GenOutput()) 获得出参的类型信息
	// NOTE: 生成的出参应当已经满足 AssertOutputType 的测试
	GenOutput() interface{}

	// AssertOutputType 应当对一个出参进行类型检查，若不通过应当 panic
	AssertOutputType(input interface{})

	// Method 是一个实现了单个方法的接口
	Interface
}

// Interface 类似于 go 中的 interface，为一组方法定义的集合，
type Interface interface {
	// HasMethod 判断某 Method 是否属于该接口
	HasMethod(method Method) bool

	// MethodByName 按方法名称查找 Method，如果找不到则返回 nil
	MethodByName(methodName string) Method

	// Methods 返回该接口所有 Method
	Methods() []Method
}

// MethodHandler 是一个方法层面的处理器，它的入参和出参是无类型的（interface{}），
// 所以需要 Method 提供类型检查
type MethodHandler interface {
	Invoke(ctx context.Context, input interface{}) (output interface{}, err error)
}

// MethodHandlerFunc 适配 MethodHandler
type MethodHandlerFunc func(context.Context, interface{}) (interface{}, error)

type defaultMethod struct {
	name       string
	inType     reflect.Type
	inFactory  func() interface{}
	outType    reflect.Type
	outFactory func() interface{}
}

type defaultInterface map[string]Method

var (
	_ Method    = (*defaultMethod)(nil)
	_ Interface = (*defaultMethod)(nil)
	_ Interface = (defaultInterface)(nil)
)

// NewMethod 定义一个新方法，inFactory 和 outFactory 必须返回非空的指针：
// 即若
//
//   input := reflect.ValueOf(inFactory())
//
// 则
//
//   input.IsValid() == true && input.Type().Kind() == reflect.Ptr && !input.IsNil()
//
// 需为真
func NewMethod(methodName string, inFactory func() interface{}, outFactory func() interface{}) Method {
	// 检查方法名
	if !IsValidMethodName(methodName) {
		panic(ErrBadMethodName)
	}

	// 检查入参工厂及其产生的入参
	if inFactory == nil {
		panic(ErrInputFactoryNil)
	}
	input := reflect.ValueOf(inFactory())
	if !input.IsValid() {
		// 若工厂生成的是 nil 接口（没有具体类型）
		panic(ErrInputNil)
	}
	inType := input.Type()
	if inType.Kind() != reflect.Ptr {
		// 若工厂生成的不是某种指针
		panic(ErrInputTypeNotPtr)
	}
	if input.IsNil() {
		// 若指针是空指针
		panic(ErrInputNilPtr)
	}

	// 检查出参工厂及其产生的出参
	if outFactory == nil {
		panic(ErrOutputFactoryNil)
	}
	output := reflect.ValueOf(outFactory())
	if !output.IsValid() {
		// 若工厂生成的是 nil 接口（没有具体类型）
		panic(ErrOutputNil)
	}
	outType := output.Type()
	if outType.Kind() != reflect.Ptr {
		// 若工厂生成的不是某种指针
		panic(ErrOutputTypeNotPtr)
	}
	if output.IsNil() {
		// 若指针是空指针
		panic(ErrOutputNilPtr)
	}

	return &defaultMethod{
		name:       methodName,
		inFactory:  inFactory,
		outFactory: outFactory,
		inType:     inType,
		outType:    outType,
	}
}

func (m *defaultMethod) Name() string {
	return m.name
}

func (m *defaultMethod) GenInput() interface{} {
	input := m.inFactory()
	m.AssertInputType(input)
	return input
}

func (m *defaultMethod) GenOutput() interface{} {
	output := m.outFactory()
	m.AssertOutputType(output)
	return output
}

func (m *defaultMethod) AssertInputType(input interface{}) {
	if reflect.TypeOf(input) != m.inType {
		panic(fmt.Errorf("Method %+q input expect %s but got %T", m.name,
			m.inType.String(), input))
	}
}

func (m *defaultMethod) AssertOutputType(output interface{}) {
	if reflect.TypeOf(output) != m.outType {
		panic(fmt.Errorf("Method %+q output expect %s but got %T", m.name,
			m.outType.String(), output))
	}
}

func (m *defaultMethod) HasMethod(method Method) bool {
	return m == method
}

func (m *defaultMethod) MethodByName(methodName string) Method {
	if methodName == m.name {
		return m
	}
	return nil
}

func (m *defaultMethod) Methods() []Method {
	return []Method{m}
}

// NewInterface 由其它 Interface 组合而成：Method 也实现了 Interface
func NewInterface(itfs ...Interface) Interface {
	i := defaultInterface{}
	for _, itf := range itfs {
		for _, method := range itf.Methods() {
			i[method.Name()] = method
		}
	}
	return i
}

func (i defaultInterface) HasMethod(method Method) bool {
	return i[method.Name()] == method
}

func (i defaultInterface) MethodByName(methodName string) Method {
	return i[methodName]
}

func (i defaultInterface) Methods() []Method {
	ms := make([]Method, 0, len(i))
	for _, m := range i {
		ms = append(ms, m)
	}
	return ms
}

// Invoke 实现 MethodHandler 接口
func (fn MethodHandlerFunc) Invoke(ctx context.Context, input interface{}) (interface{}, error) {
	return fn(ctx, input)
}

var (
	methodNameRegexp = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_]*)(\.([a-zA-Z][a-zA-Z0-9_]*))*$`)
)

// IsValidMethodName 返回一个字符串是否是合法的方法名：xxx.xxx.xxx
// 其中 xxx 为合法的变量名，即字母开头，后续为字母数字或下划线
func IsValidMethodName(methodName string) bool {
	return methodNameRegexp.MatchString(methodName)
}
