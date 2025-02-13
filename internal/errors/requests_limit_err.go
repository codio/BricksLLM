package errors

type RequestsLimitError struct {
	message string
}

func NewRequestsLimitError(msg string) *RequestsLimitError {
	return &RequestsLimitError{
		message: msg,
	}
}

func (rle *RequestsLimitError) Error() string {
	return rle.message
}

func (rle *RequestsLimitError) RequestsLimit() {}
