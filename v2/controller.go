package formigo

import (
	"context"
	"sync"
)

type controller struct {
	errorConfig  ErrorConfiguration
	errorCounter int
	mutex        sync.Mutex
	stopOnce     sync.Once
	stopFunc     context.CancelCauseFunc
}

func (c *controller) reportError(err error) {
	c.errorConfig.ReportFunc(err)
}

func newController(errorConfig ErrorConfiguration) *controller {
	return &controller{
		errorConfig: errorConfig,
	}
}
