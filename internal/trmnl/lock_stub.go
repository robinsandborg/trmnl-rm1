//go:build !linux && !darwin

package trmnl

import "errors"

func acquireCycleLock(path string) (func(), bool, error) {
	return nil, false, errors.New("cycle locking unsupported on this platform")
}
