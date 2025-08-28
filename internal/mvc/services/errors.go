package services

import "errors"

// 服务层错误定义
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenInvalid       = errors.New("token invalid")

	ErrDroneNotFound     = errors.New("drone not found")
	ErrDroneExists       = errors.New("drone already exists")
	ErrDroneNotAvailable = errors.New("drone not available")
	ErrDroneInUse        = errors.New("drone is in use")

	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskExists         = errors.New("task already exists")
	ErrTaskNotRunning     = errors.New("task is not running")
	ErrTaskAlreadyRunning = errors.New("task is already running")
	ErrTaskCannotStart    = errors.New("task cannot start")

	ErrAlertNotFound        = errors.New("alert not found")
	ErrAlertAlreadyResolved = errors.New("alert already resolved")

	ErrInvalidData      = errors.New("invalid data")
	ErrPermissionDenied = errors.New("permission denied")
	ErrInternalError    = errors.New("internal error")
)
