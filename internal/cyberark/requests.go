package cyberark

import "fmt"

func (a *AuthManager) GetIncomingRequests() ([]IncomingRequest, error) {
	var resp IncomingRequestsResponse
	err := a.DoRequestWithReAuth("GET", "/PasswordVault/API/IncomingRequests?onlywaiting=true", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.IncomingRequests, nil
}

func (a *AuthManager) GetIncomingRequestDetail(requestID string) (*IncomingRequestDetail, error) {
	var resp IncomingRequestDetail
	endpoint := fmt.Sprintf("/PasswordVault/API/IncomingRequests/%s", requestID)
	err := a.DoRequestWithReAuth("GET", endpoint, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (a *AuthManager) ConfirmRequest(requestID string, reason string) error {
	payload := ActionRequest{Reason: reason}
	endpoint := fmt.Sprintf("/PasswordVault/API/IncomingRequests/%s/Confirm", requestID)
	return a.DoRequestWithReAuth("POST", endpoint, payload, nil)
}

func (a *AuthManager) RejectRequest(requestID string, reason string) error {
	payload := ActionRequest{Reason: reason}
	endpoint := fmt.Sprintf("/PasswordVault/API/IncomingRequests/%s/Reject", requestID)
	return a.DoRequestWithReAuth("POST", endpoint, payload, nil)
}

func (a *AuthManager) BulkConfirmRequests(requestIDs []string, reason string) (*BulkActionResponse, error) {
	payload := BulkActionRequest{
		RequestIDs: requestIDs,
		Reason:     reason,
	}
	var resp BulkActionResponse
	err := a.DoRequestWithReAuth("POST", "/PasswordVault/API/IncomingRequests/Confirm", payload, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (a *AuthManager) BulkRejectRequests(requestIDs []string, reason string) (*BulkActionResponse, error) {
	payload := BulkActionRequest{
		RequestIDs: requestIDs,
		Reason:     reason,
	}
	var resp BulkActionResponse
	err := a.DoRequestWithReAuth("POST", "/PasswordVault/API/IncomingRequests/Reject", payload, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
