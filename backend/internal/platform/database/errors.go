package database

import "errors"

var ErrDisabled = errors.New("adapter cơ sở dữ liệu đang tắt")
