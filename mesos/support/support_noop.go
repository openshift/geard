// +build !mesos

package support

func HasBinaries() bool {
	return true
}

func InitializeBinaries() error {
	return nil
}
