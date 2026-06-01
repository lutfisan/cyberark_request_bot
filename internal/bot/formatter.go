package bot

import (
	"fmt"
	"strings"
	"time"

	"cybarbot/internal/cyberark"
)

func formatRequestList(requests []cyberark.IncomingRequest, page, totalPages int) string {
	if len(requests) == 0 {
		return "✅ No pending requests"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Pending Requests (Page %d / %d)\n", page, totalPages))
	sb.WriteString("─────────────────────────────────────────────\n")

	for _, req := range requests {
		// Just standardizing on Unix timestamp since exact CyberArk payload can vary
		// Assume CreationDate is Unix milliseconds if very large, otherwise seconds.
		// For simplicity, formatting as a string placeholder if conversion is tricky,
		// but let's try basic format.
		creationTime := time.Unix(req.CreationDate, 0).UTC().Format("2006-01-02 15:04 MST")
		
		sb.WriteString(fmt.Sprintf("[%s] %s → %s | %s\n", req.RequestID, req.RequesterUserName, req.SafeName, creationTime))
	}
	sb.WriteString("─────────────────────────────────────────────\n")
	
	return sb.String()
}

func formatRequestDetail(req *cyberark.IncomingRequestDetail) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Request Details: %s\n", req.RequestID))
	sb.WriteString("─────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("Requester   : %s\n", req.RequesterUserName))
	sb.WriteString(fmt.Sprintf("Account     : %s\n", req.AccountName))
	sb.WriteString(fmt.Sprintf("Safe        : %s\n", req.SafeName))
	sb.WriteString(fmt.Sprintf("Access Type : %s\n", req.AccessType))
	
	expiryTime := time.Unix(req.ExpirationDate, 0).UTC().Format("2006-01-02 15:04 MST")
	sb.WriteString(fmt.Sprintf("Expires At  : %s\n", expiryTime))
	sb.WriteString(fmt.Sprintf("Reason      : %s\n", req.Reason))
	sb.WriteString(fmt.Sprintf("Status      : %s\n", req.Status))
	sb.WriteString("─────────────────────────────────────────────\n")
	sb.WriteString("Workflow Steps:\n")
	for _, step := range req.ConfirmSteps {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", step.Reviewer, step.Status))
	}
	return sb.String()
}

func formatNotification(req cyberark.IncomingRequestDetail) string {
	var sb strings.Builder
	sb.WriteString("──────────────────────────────────────────────────\n")
	sb.WriteString("🔔 New Access Request\n\n")
	sb.WriteString(fmt.Sprintf("Request ID   : %s\n", req.RequestID))
	sb.WriteString(fmt.Sprintf("Requester    : %s\n", req.RequesterUserName))
	sb.WriteString(fmt.Sprintf("Safe         : %s\n", req.SafeName))
	sb.WriteString(fmt.Sprintf("Account      : %s\n", req.AccountName))
	sb.WriteString(fmt.Sprintf("Access Type  : %s\n", req.AccessType))
	
	expiryTime := time.Unix(req.ExpirationDate, 0).UTC().Format("2006-01-02 15:04 MST")
	creationTime := time.Unix(req.CreationDate, 0).UTC().Format("2006-01-02 15:04 MST")
	
	sb.WriteString(fmt.Sprintf("Expires At   : %s\n", expiryTime))
	sb.WriteString(fmt.Sprintf("Reason       : %s\n", req.Reason))
	sb.WriteString(fmt.Sprintf("Received At  : %s\n", creationTime))
	sb.WriteString("──────────────────────────────────────────────────\n")
	return sb.String()
}
