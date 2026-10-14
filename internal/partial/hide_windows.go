//go:build windows

package partial

import "syscall"

// hide sets the hidden attribute, keeping the attributes already there.
func hide(path string) error {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes, err := syscall.GetFileAttributes(name)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(name, attributes|syscall.FILE_ATTRIBUTE_HIDDEN)
}
