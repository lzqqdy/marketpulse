package users

import "errors"

var (
	ErrDisabled           = errors.New("users: module disabled")
	ErrInvalidCredentials = errors.New("users: invalid phone or password")
	ErrUnauthorized       = errors.New("users: unauthorized")
	ErrNotFound           = errors.New("users: not found")
	ErrInvalidInput       = errors.New("users: invalid input")
	ErrPhoneExists        = errors.New("users: phone already exists")
	ErrWrongPassword      = errors.New("users: wrong password")
	ErrRateLimited        = errors.New("users: rate limited")
	ErrLoginLocked        = errors.New("users: login temporarily locked")
)
