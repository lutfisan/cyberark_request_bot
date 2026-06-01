package cyberark

// CyberArk uses custom time formatting in its JSON responses sometimes, 
// but for standard models we'll map fields directly.

type LogonResponse string // API returns raw string

type IncomingRequest struct {
	RequestID         string `json:"RequestID"`
	RequestorUserName string `json:"RequestorUserName"`
	RequesterUserName string `json:"RequesterUserName"`
	SafeName          string `json:"SafeName"`
	AccountName       string `json:"AccountName"`
	AccountAddress    string `json:"AccountAddress"`
	Operation         string `json:"Operation"`
	Status            int    `json:"Status"`
	CreationDate      int64  `json:"CreationDate"`
	ExpirationDate    int64  `json:"ExpirationDate"`
	AccessFrom        int64  `json:"AccessFrom"`
	AccessTo          int64  `json:"AccessTo"`
	UserReason        string `json:"UserReason"`
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
	RequestID         string        `json:"RequestID"`
	RequestorUserName string        `json:"RequestorUserName"`
	RequesterUserName string        `json:"RequesterUserName"`
	SafeName          string        `json:"SafeName"`
	AccountName       string        `json:"AccountName"`
	AccountAddress    string        `json:"AccountAddress"`
	AccessType        string        `json:"AccessType"`
	Operation         string        `json:"Operation"`
	CreationDate      int64         `json:"CreationDate"`
	ExpirationDate    int64         `json:"ExpirationDate"`
	AccessFrom        int64         `json:"AccessFrom"`
	AccessTo          int64         `json:"AccessTo"`
	Reason            string        `json:"Reason"`
	UserReason        string        `json:"UserReason"`
	Status            int           `json:"Status"`
	ConfirmSteps      []ConfirmStep `json:"ConfirmSteps"`
}

type ActionRequest struct {
	Reason string `json:"Reason"`
}

type BulkItem struct {
	RequestID string `json:"RequestID"`
	Reason    string `json:"Reason"`
}

type BulkActionRequest struct {
	BulkItems []BulkItem `json:"BulkItems"`
}

type BulkActionResponse struct {
	Successful []string `json:"Successful"`
	Failed     []struct {
		RequestID string `json:"RequestID"`
		Error     string `json:"Error"`
	} `json:"Failed"`
}
