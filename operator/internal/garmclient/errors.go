/*
Copyright 2026 Igor Kachurin.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package garmclient

// codedError is the duck-typed shape of every go-swagger generated
// *Default error: it has a Code() returning the HTTP status. We don't
// import each per-operation type — go-swagger emits one per op.
type codedError interface {
	Code() int
}

func httpStatus(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	if ce, ok := err.(codedError); ok {
		return ce.Code(), true
	}
	return 0, false
}

// IsNotFound reports whether err represents a 404 from GARM.
func IsNotFound(err error) bool {
	code, ok := httpStatus(err)
	return ok && code == 404
}

// IsConflict reports whether err represents a 409 (e.g. trying to
// create an endpoint whose name already exists).
func IsConflict(err error) bool {
	code, ok := httpStatus(err)
	return ok && code == 409
}

func isUnauthorized(err error) bool {
	code, ok := httpStatus(err)
	return ok && code == 401
}
