package main

import (
	"fmt"

	"github.com/tyke/tyke/tyke/common"
	"github.com/tyke/tyke/tyke/core"
)

func main() {
	str := "hello world "

	response := core.AcquireResponse()
	defer core.ReleaseResponse(response)
	request := core.AcquireRequest()
	defer core.ReleaseRequest(request)

	request.SetModule("test").SetRoute("/test/hello").SetContent(common.ContentTypeText, []byte(str))

	expected := request.Send("39649d81-81c5-4f6e-b6a9-e768b55063be", response)
	if !expected.HasValue() {
		fmt.Printf("expected error: %s", expected.Err)
	}

	contentType, content := response.GetContent()
	fmt.Printf("resp_content_type: %s\n", contentType)
	fmt.Printf("resp_content: %s\n", string(content))
}
