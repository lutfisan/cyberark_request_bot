package bot

import (
	"fmt"
	"strings"
	"time"

	"cybarbot/internal/cyberark"
)

func getRequester(reqor string) string {
	if reqor != "" {
		return reqor
	}
	return "Unknown"
}

func getAccountStr(details cyberark.AccountDetails, operation string) (string, string) {
	name := details.Properties.UserName
	addr := details.Properties.Address

	if name == "" && addr == "" && operation != "" {
		// Attempt to extract from operation like "Connect with ca_adm on 10.206.48.197"
		parts := strings.Split(operation, " on ")
		if len(parts) == 2 {
			addr = strings.TrimSpace(parts[1])
			nameParts := strings.Split(parts[0], "with ")
			if len(nameParts) == 2 {
				name = strings.TrimSpace(nameParts[1])
			}
		} else {
			name = operation
		}
	}

	if name == "" {
		name = "Unknown"
	}
	if addr == "" {
		addr = "Unknown"
	}
	return name, addr
}

var tzLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		tzLocation = time.Local
	} else {
		tzLocation = loc
	}
}

func getTimeFrame(accessFrom, accessTo, creationDate, expirationDate int64) string {
	var from, to int64
	if accessFrom > 0 && accessTo > 0 {
		from = accessFrom
		to = accessTo
	} else {
		from = creationDate
		to = expirationDate
	}
	
	fromStr := "Unknown"
	toStr := "Unknown"
	
	if from > 0 {
		fromStr = time.Unix(from, 0).In(tzLocation).Format("2006-01-02 15:04")
	}
	if to > 0 {
		toStr = time.Unix(to, 0).In(tzLocation).Format("2006-01-02 15:04")
	}
	
	return fmt.Sprintf("%s to %s", fromStr, toStr)
}

func formatTime(t int64) string {
	if t <= 0 {
		return "Unknown"
	}
	return time.Unix(t, 0).In(tzLocation).Format("2006-01-02 15:04 MST")
}

func formatRequestList(requests []cyberark.IncomingRequest, page, totalPages int) string {
	if len(requests) == 0 {
		return "✅ No pending requests"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Pending Requests (Page %d / %d)\n", page, totalPages))
	sb.WriteString("─────────────────────────────────────────────\n")

	for _, req := range requests {
		requester := getRequester(req.RequestorUserName)
		_, addr := getAccountStr(req.AccountDetails, req.Operation)
		timeframe := getTimeFrame(req.AccessFrom, req.AccessTo, req.CreationDate, req.ExpirationDate)

		sb.WriteString(fmt.Sprintf("[%s] %s → %s | %s | %s\n", req.RequestID, requester, req.SafeName, addr, timeframe))
	}
	sb.WriteString("─────────────────────────────────────────────\n")
	
	return sb.String()
}

func formatRequestDetail(req *cyberark.IncomingRequestDetail) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Request Details: %s\n", req.RequestID))
	sb.WriteString("─────────────────────────────────────────────\n")
	
	requester := getRequester(req.RequestorUserName)
	accountName, accountAddr := getAccountStr(req.AccountDetails, req.Operation)
	timeframe := getTimeFrame(req.AccessFrom, req.AccessTo, req.CreationDate, req.ExpirationDate)

	sb.WriteString(fmt.Sprintf("Requester    : %s\n", requester))
	sb.WriteString(fmt.Sprintf("Address      : %s\n", accountAddr))
	sb.WriteString(fmt.Sprintf("Account User : %s\n", accountName))
	sb.WriteString(fmt.Sprintf("Account Name : %s\n", req.AccountDetails.Properties.Name))
	sb.WriteString(fmt.Sprintf("Safe         : %s\n", req.SafeName))
	sb.WriteString(fmt.Sprintf("Access Type  : %s\n", req.AccessType))
	sb.WriteString(fmt.Sprintf("Time Frame   : %s\n", timeframe))
	sb.WriteString(fmt.Sprintf("Expires At   : %s\n", formatTime(req.ExpirationDate)))
	
	reqReason := req.RequestorReason
	if reqReason == "" {
		reqReason = "None"
	}
	userReason := req.UserReason
	if userReason == "" {
		userReason = "None"
	}
	
	sb.WriteString(fmt.Sprintf("Req Reason   : %s\n", reqReason))
	sb.WriteString(fmt.Sprintf("User Reason  : %s\n", userReason))
	sb.WriteString(fmt.Sprintf("Status       : %v\n", req.Status))
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
	
	requester := getRequester(req.RequestorUserName)
	accountName, accountAddr := getAccountStr(req.AccountDetails, req.Operation)
	timeframe := getTimeFrame(req.AccessFrom, req.AccessTo, req.CreationDate, req.ExpirationDate)
	reason := req.RequestorReason
	if reason == "" {
		reason = req.UserReason
	}
	if reason == "" {
		reason = "None"
	}

	sb.WriteString(fmt.Sprintf("Request ID   : %s\n", req.RequestID))
	sb.WriteString(fmt.Sprintf("Requester    : %s\n", requester))
	sb.WriteString(fmt.Sprintf("Safe         : %s\n", req.SafeName))
	sb.WriteString(fmt.Sprintf("Account User : %s\n", accountName))
	sb.WriteString(fmt.Sprintf("Account Addr : %s\n", accountAddr))
	sb.WriteString(fmt.Sprintf("Access Type  : %s\n", req.AccessType))
	sb.WriteString(fmt.Sprintf("Time Frame   : %s\n", timeframe))
	sb.WriteString(fmt.Sprintf("Reason       : %s\n", reason))
	
	creationTime := formatTime(req.CreationDate)
	sb.WriteString(fmt.Sprintf("Received At  : %s\n", creationTime))
	sb.WriteString("──────────────────────────────────────────────────\n")
	return sb.String()
}
