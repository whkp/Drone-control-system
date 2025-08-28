package controllers

import "errors"

// 控制器层错误定义
var (
	ErrUserIDNotFound   = errors.New("user ID not found in context")
	ErrInvalidUserID    = errors.New("invalid user ID format")
	ErrUserRoleNotFound = errors.New("user role not found in context")
	ErrInvalidUserRole  = errors.New("invalid user role format")
	ErrInvalidID        = errors.New("invalid ID parameter")
	ErrBindingFailed    = errors.New("request data binding failed")
)
