// +build selinux

package selinux

import (
	se "github.com/openshift/geard/selinux/library"
)

func RestoreCon(path string, recursive bool) error {
	return se.RestoreCon(path, recursive)
}
