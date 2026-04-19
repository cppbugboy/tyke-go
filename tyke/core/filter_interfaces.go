package core

type RequestFilter interface {
	Before(request *TykeRequest, response *TykeResponse) bool
	After(request *TykeRequest, response *TykeResponse) bool
}

type ResponseFilter interface {
	Before(response *TykeResponse) bool
	After(response *TykeResponse) bool
}
