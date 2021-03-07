// Code generated by msgraph.go/gen DO NOT EDIT.

package msgraph

import "context"

//
type WorkbookFunctionsPowerRequestBuilder struct{ BaseRequestBuilder }

// Power action undocumented
func (b *WorkbookFunctionsRequestBuilder) Power(reqObj *WorkbookFunctionsPowerRequestParameter) *WorkbookFunctionsPowerRequestBuilder {
	bb := &WorkbookFunctionsPowerRequestBuilder{BaseRequestBuilder: b.BaseRequestBuilder}
	bb.BaseRequestBuilder.baseURL += "/power"
	bb.BaseRequestBuilder.requestObject = reqObj
	return bb
}

//
type WorkbookFunctionsPowerRequest struct{ BaseRequest }

//
func (b *WorkbookFunctionsPowerRequestBuilder) Request() *WorkbookFunctionsPowerRequest {
	return &WorkbookFunctionsPowerRequest{
		BaseRequest: BaseRequest{baseURL: b.baseURL, client: b.client, requestObject: b.requestObject},
	}
}

//
func (r *WorkbookFunctionsPowerRequest) Post(ctx context.Context) (resObj *WorkbookFunctionResult, err error) {
	err = r.JSONRequest(ctx, "POST", "", r.requestObject, &resObj)
	return
}
