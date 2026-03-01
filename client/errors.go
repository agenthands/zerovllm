package vllmipc

import "errors"

var (
	ErrConstraintViolated = errors.New("vllmipc: constraint violated")
	ErrEngineTimeout      = errors.New("vllmipc: timeout waiting for engine response")
	ErrZeroMQClosed       = errors.New("vllmipc: zeromq client closed")
	ErrInvalidResponse    = errors.New("vllmipc: invalid response from server")
)
