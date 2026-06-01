package cyberark

// CyberArk uses custom time formatting in its JSON responses sometimes, 
// but for standard models we'll map fields directly.

type LogonResponse string // API returns raw string

type IncomingRequest struct {
	RequestID        string `json:"RequestID"`
	RequesterUserName string `json:"RequesterUserName"`
	SafeName         string `json:"SafeName"`
	AccountName      string `json:"AccountName"`
	Status           int    `json:"Status"`
	CreationDate     int64  `json:"CreationDate"` // Assuming Unix timestamp or similar, might need to adjust based on exact API format
}

type IncomingRequestsResponse struct {
	IncomingRequests []IncomingRequest `json:"IncomingRequests"`
	Total            int               `json:"Total"`
}

type ConfirmStep struct {
	StepID   int    `json:"StepID"`
	Reviewer string `json:"Reviewer"`
	Status   string `json:"Status"`
}

type IncomingRequestDetail struct {
	RequestID        string        `json:"RequestID"`
	RequesterUserName string        `json:"RequesterUserName"`
	SafeName         string        `json:"SafeName"`
	AccountName      string        `json:"AccountName"`
	AccessType       string        `json:"AccessType"`
	ExpirationDate   int64         `json:"ExpirationDate"`
	Reason           string        `json:"Reason"`
	Status           int           `json:"Status"`
	ConfirmSteps     []ConfirmStep `json:"ConfirmSteps"`
	CreationDate     int64         `json:"CreationDate"`
}

type ActionRequest struct {
	Reason string `json:"Reason"`
}

type BulkActionRequest struct {
	RequestIDs []string `json:"RequestIDs"`
	Reason     string   `json:"Reason"`
}

type BulkActionResponse struct {
	Successful []string `json:"Successful"`
	Failed     []struct {
		RequestID string `json:"RequestID"`
		Error     string `json:"Error"`
	} `json:"Failed"`
}
