package api

type HTTPError struct {
	StatusCode int
	Message    string
	ErrorLog   error
}

func (e *HTTPError) Error() string {
	return e.Message
}

type ApiError struct {
	Error string `json:"message"`
}