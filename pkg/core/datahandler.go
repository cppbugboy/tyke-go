package core

type DataHandler struct {
}

func NewDataHandler() *DataHandler {
	return &DataHandler{}
}

func (h *DataHandler) OnRequestData(data []byte, sendResponse func([]byte) bool) []byte {
	req, _, err := DecodeRequest(data)
	if err != nil {
		LogError("DataHandler: decode request failed: %v", err)
		return nil
	}
	defer ReleaseRequest(req)

	resp := AcquireResponse()
	defer ReleaseResponse(resp)

	DispatchRequest(req, resp)

	encoded, err := EncodeResponse(resp)
	if err != nil {
		LogError("DataHandler: encode response failed: %v", err)
		return nil
	}

	return encoded
}

func (h *DataHandler) OnResponseData(data []byte) {
	resp, _, err := DecodeResponse(data)
	if err != nil {
		LogError("DataHandler: decode response failed: %v", err)
		return
	}
	defer ReleaseResponse(resp)

	DispatchResponse(resp)
}
