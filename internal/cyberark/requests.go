package cyberark

import (
	"encoding/json"
	"fmt"
)

func (a *AuthManager) GetIncomingRequests() ([]IncomingRequest, error) {
	var rawJSON []byte
	err := a.DoRequestWithReAuth("GET", "/PasswordVault/API/IncomingRequests?onlywaiting=true", nil, &rawJSON)
	if err != nil {
		return nil, err
	}
	fmt.Println("DEBUG_JSON_RESPONSE:", string(rawJSON))
	
	var resp IncomingRequestsResponse
	if err := json.Unmarshal(rawJSON, &resp); err != nil {
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
	resp := &BulkActionResponse{
		Successful: make([]string, 0),
		Failed: make([]struct {
			RequestID string `json:"RequestID"`
			Error     string `json:"Error"`
		}, 0),
	}

	for _, id := range requestIDs {
		err := a.ConfirmRequest(id, reason)
		if err != nil {
			resp.Failed = append(resp.Failed, struct {
				RequestID string `json:"RequestID"`
				Error     string `json:"Error"`
			}{RequestID: id, Error: err.Error()})
		} else {
			resp.Successful = append(resp.Successful, id)
		}
	}

	if len(resp.Failed) > 0 {
		return resp, fmt.Errorf("some requests failed to confirm")
	}

	return resp, nil
}

func (a *AuthManager) BulkRejectRequests(requestIDs []string, reason string) (*BulkActionResponse, error) {
	resp := &BulkActionResponse{
		Successful: make([]string, 0),
		Failed: make([]struct {
			RequestID string `json:"RequestID"`
			Error     string `json:"Error"`
		}, 0),
	}

	for _, id := range requestIDs {
		err := a.RejectRequest(id, reason)
		if err != nil {
			resp.Failed = append(resp.Failed, struct {
				RequestID string `json:"RequestID"`
				Error     string `json:"Error"`
			}{RequestID: id, Error: err.Error()})
		} else {
			resp.Successful = append(resp.Successful, id)
		}
	}

	if len(resp.Failed) > 0 {
		return resp, fmt.Errorf("some requests failed to reject")
	}

	return resp, nil
}
