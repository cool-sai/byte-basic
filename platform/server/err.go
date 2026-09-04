package main

import (
	"connectrpc.com/connect"
)

func invalid(err error) error {
	return connect.NewError(connect.CodeInvalidArgument, err)
}

func unauth(err error) error {
	return connect.NewError(connect.CodeUnauthenticated, err)
}

func notFound(err error) error {
	return connect.NewError(connect.CodeNotFound, err)
}

func internal(err error) error {
	return connect.NewError(connect.CodeInternal, err)
}


