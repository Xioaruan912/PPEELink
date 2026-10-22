//go:build windows

package xraycore

import "errors"

func execSelf(string) error {
	return errors.New("Windows agent 不支持自更新")
}
