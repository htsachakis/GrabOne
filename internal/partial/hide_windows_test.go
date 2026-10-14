//go:build windows

package partial

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestTheFolderForPartialFilesIsHidden(t *testing.T) {
	output := t.TempDir()
	if _, err := Prepare(output, "job-1"); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	name, _ := syscall.UTF16PtrFromString(filepath.Join(output, DirName))
	attributes, err := syscall.GetFileAttributes(name)
	if err != nil {
		t.Fatal(err)
	}
	if attributes&syscall.FILE_ATTRIBUTE_HIDDEN == 0 {
		t.Error("the folder holding partial files is not hidden")
	}
}
