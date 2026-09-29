package formigo

type controller struct {
	reportFunc func(error)
}

func (c *controller) reportError(err error) {
	if c.reportFunc != nil {
		c.reportFunc(err)
	}
}

func newController(reportFunc func(error)) *controller {
	return &controller{
		reportFunc: reportFunc,
	}
}
