package libsvc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMethodName(t *testing.T) {
	a := assert.New(t)
	for methodName, expectValid := range map[string]bool{
		"a":      true,
		"Z":      true,
		"0":      false,
		"x01010": true,
		".x":     false,
		"x.":     false,
		"x.y":    true,
		"x.y.z":  true,
	} {
		valid := IsValidMethodName(methodName)
		a.Equal(expectValid, valid, "IsValidMethodName(%+q) expect %v but got %v", methodName, expectValid, valid)
	}
}

func TestNewMethod(t *testing.T) {
	a := assert.New(t)

	a.Panics(func() {
		NewMethod(
			"bad.method.*",
			func() interface{} { return &struct{}{} },
			func() interface{} { return &struct{}{} },
		)
	}, "Expect panic for bad method name")

	a.Panics(func() {
		NewMethod(
			"nil.input.factory",
			nil,
			func() interface{} { return &struct{}{} },
		)
	}, "Expect panic since input factory is nil")

	a.Panics(func() {
		NewMethod(
			"nil.output.factory",
			func() interface{} { return &struct{}{} },
			nil,
		)
	}, "Expect panic since output factory is nil")

	a.Panics(func() {
		NewMethod(
			"nil.input",
			func() interface{} { return error(nil) },
			func() interface{} { return &struct{}{} },
		)
	}, "Expect panic since input factory returns nil interface (any interface)")

	a.Panics(func() {
		NewMethod(
			"nil.output",
			func() interface{} { return &struct{}{} },
			func() interface{} { return error(nil) },
		)
	}, "Expect panic since output factory returns nil interface (any interface)")

	a.Panics(func() {
		NewMethod(
			"input.not.ptr",
			func() interface{} { return 1000 },
			func() interface{} { return &struct{}{} },
		)
	}, "Expect panic since input factory returns non-ptr")

	a.Panics(func() {
		NewMethod(
			"output.not.ptr",
			func() interface{} { return &struct{}{} },
			func() interface{} { return 1000 },
		)
	}, "Expect panic since output factory returns non-ptr")

	a.Panics(func() {
		NewMethod(
			"input.nil.ptr",
			func() interface{} { return (*struct{})(nil) },
			func() interface{} { return &struct{}{} },
		)
	}, "Expect panic since input factory returns nil ptr")

	a.Panics(func() {
		NewMethod(
			"output.nil.ptr",
			func() interface{} { return &struct{}{} },
			func() interface{} { return (*struct{})(nil) },
		)
	}, "Expect panic since output factory returns nil ptr")

}
