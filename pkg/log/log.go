//go:build !windows
// +build !windows

package log

import (
	"github.com/go-kit/log"
	"github.com/prometheus/common/promlog"
)

func InitLogger(cfg *promlog.Config) (log.Logger, error) {
	return promlog.New(cfg), nil
}
