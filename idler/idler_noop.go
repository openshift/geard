// +build !idler

package idler

import (
	"github.com/smarterclayton/geard/containers"
)

func StartIdler(pDockerSocket *string, pHostIp *string) {
}

func RegisterApp(id containers.Identifier, started bool) {
}

func StopApplication(id containers.Identifier) {

}

func StartApplication(id containers.Identifier) {

}

func DeleteApplication(id containers.Identifier) {

}
