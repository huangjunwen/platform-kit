package libsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	noopMethod = NewMethod("noop", func() interface{} { return &struct{}{} }, func() interface{} { return &struct{}{} })
)

func TestLocalService(t *testing.T) {
	a := assert.New(t)

	a.Panics(func() {
		NewLocalService("bad.service.name.*")
	}, "Expect panic since bad service name calling NewLocalService")

	a.Panics(func() {
		NewLocalService("odd.params", 1)
	}, "Expect panic since odd params calling NewLocalService")

	a.Panics(func() {
		NewLocalService("not.method", nil, nil)
	}, "Expect panic since non method in calling NewLocalService")

	a.Panics(func() {
		NewLocalService("not.handler", noopMethod, 100)
	}, "Expect panic since non method handler in calling NewLocalService")

	a.Panics(func() {
		svc := NewLocalService(
			"bad.output.type",
			noopMethod,
			func(_ context.Context, input, output interface{}) error {
				return nil
			},
		)
		svc.Invoke(context.Background(), noopMethod, &struct{}{}, 100)
	}, "Expect panic since handler's output does not match method's")

	a.Panics(func() {
		svc := NewLocalService(
			"bad.input.type",
			noopMethod,
			func(_ context.Context, input, output interface{}) error {
				return nil
			},
		)
		svc.Invoke(context.Background(), noopMethod, 100, &struct{}{})
	}, "Expect panic since input does not match method's")

}
