//go:build !windows

package partial

// hide has nothing to do here: a name starting with a dot is what hides a
// folder on these systems, and DirName has one.
func hide(path string) error {
	_ = path
	return nil
}
