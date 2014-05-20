package cleanup

import (
	"bytes"
	"log"
)

func newContext(dryrun bool, repair bool) (*CleanerContext, *bytes.Buffer, *bytes.Buffer) {
	info := &bytes.Buffer{}

	error := &bytes.Buffer{}
	logInfo := log.New(info, "INFO: ", log.Ldate|log.Ltime)
	logError := log.New(error, "ERROR: ", log.Ldate|log.Ltime)

	return &CleanerContext{DryRun: dryrun, Repair: repair, LogInfo: logInfo, LogError: logError}, info, error
}
