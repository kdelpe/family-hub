//go:build !linux && !darwin

package admin

func discardInput(int) error { return nil }
