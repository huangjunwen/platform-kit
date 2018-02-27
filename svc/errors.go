package libsvc

import (
	"errors"
)

var (
	ErrBadMethodName     = errors.New("Bad method name")
	ErrInputFactoryNil   = errors.New("InputFactory is nil")
	ErrInputNil          = errors.New("Input is nil")
	ErrInputTypeNotPtr   = errors.New("Input is not ptr")
	ErrInputNilPtr       = errors.New("Input is nil ptr")
	ErrOutputFactoryNil  = errors.New("OutputFactory is nil")
	ErrOutputNil         = errors.New("Output is nil")
	ErrOutputTypeNotPtr  = errors.New("Output is not ptr")
	ErrOutputNilPtr      = errors.New("Output is nil ptr")
	ErrBadSvcName        = errors.New("Bad service name")
	ErrAltIsInprocClient = errors.New("Alt client should not be the inproc client")
	ErrMethodNotFound    = errors.New("Method not found or not implemented")
	ErrSvcNotFound       = errors.New("Service not found")
	ErrSvcNameConflict   = errors.New("Service name conflict (duplicated)")
	ErrMethodHandlerPair = errors.New("Expect Method and MethodHandler pairs")
)
