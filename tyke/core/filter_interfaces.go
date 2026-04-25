package core

type RequestFilter interface {
	Before(request *Request, response *Response) bool
	After(request *Request, response *Response) bool
}

type ResponseFilter interface {
	Before(response *Response) bool
	After(response *Response) bool
}
