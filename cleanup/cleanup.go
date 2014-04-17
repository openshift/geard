package cleanup

import (
	"log"
)

type CleanerContext struct {
	DryRun        bool
	Repair        bool
	LogInfo      *log.Logger
	LogError     *log.Logger
}

type Cleaner interface {
	Clean(context *CleanerContext)
}

var (
	cleanupList []Cleaner
	LogInfo      *log.Logger
	LogError     *log.Logger
)

func init() {

	cleanupList = []Cleaner{}
}

func Clean(ctx *CleanerContext) {
	for _, r := range cleanupList {
		r.Clean(ctx)
	}
}

func AddCleaner(cleanup Cleaner) {
	cleanupList = append(cleanupList, cleanup)
}
