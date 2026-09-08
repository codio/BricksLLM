package errors

type CostLimitError struct {
	message  string
	timeUnit string
}

func NewCostLimitError(msg string, timeUnit ...string) *CostLimitError {
	cle := &CostLimitError{
		message: msg,
	}
	if len(timeUnit) != 0 {
		cle.timeUnit = timeUnit[0]
	}
	return cle
}

func (cle *CostLimitError) Error() string {
	return cle.message
}

func (cle *CostLimitError) TimeUnit() string {
	return cle.timeUnit
}

func (rle *CostLimitError) CostLimit() {}
