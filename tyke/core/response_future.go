package core

type ResponseFuture struct {
	msgUuid string
	ch      chan *TykeResponse
}

func NewResponseFuture(msgUuid string, ch chan *TykeResponse) ResponseFuture {
	return ResponseFuture{msgUuid: msgUuid, ch: ch}
}

func (f *ResponseFuture) GetResponse() *TykeResponse {
	return <-f.ch
}
