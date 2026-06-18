package client

// Builders construct a validated EmailRequest for a specific messageId. Each
// owns the exact messageId and context keys the template expects (see the
// email-templates repo's template-variables.json), so producers get a typed,
// typo-proof API instead of hand-building a map[string]any.
//
// Add a new email by adding a builder + Args struct here; callers of other
// builders are unaffected. The keys MUST match the template's variables — keep
// these in sync with template-variables.json (a future codegen step could
// generate this file from that manifest).
//
// For a template without a typed builder yet, use Message() — the untyped
// escape hatch.

// Message builds a request for an arbitrary messageId with a raw context map.
// Prefer a typed builder when one exists; use this only for templates not yet
// covered or for one-off/experimental sends.
func Message(messageId string, to To, context map[string]any) EmailRequest {
	if context == nil {
		context = map[string]any{}
	}
	return EmailRequest{
		MessageId:  messageId,
		Recipients: []Recipient{to},
		Context:    context,
	}
}

// --- typed builders ---------------------------------------------------------

// DatasetPublicationAcceptedArgs are the variables for the
// "dataset-publication-accepted" template.
type DatasetPublicationAcceptedArgs struct {
	DatasetName  string
	ReviewerName string
	Date         string
}

// DatasetPublicationAccepted notifies a dataset owner that their dataset was
// accepted for publication.
func DatasetPublicationAccepted(to To, args DatasetPublicationAcceptedArgs) EmailRequest {
	return Message("dataset-publication-accepted", to, map[string]any{
		"datasetName":  args.DatasetName,
		"reviewerName": args.ReviewerName,
		"date":         args.Date,
	})
}

// ChangeOfDatasetOwnerArgs are the variables for the "change-of-dataset-owner"
// template.
type ChangeOfDatasetOwnerArgs struct {
	DatasetName        string
	DatasetNodeId      string
	Host               string
	OrganizationName   string
	OrganizationNodeId string
	PreviousOwnerName  string
}

// ChangeOfDatasetOwner notifies a user that they are now the owner of a dataset.
func ChangeOfDatasetOwner(to To, args ChangeOfDatasetOwnerArgs) EmailRequest {
	return Message("change-of-dataset-owner", to, map[string]any{
		"datasetName":        args.DatasetName,
		"datasetNodeId":      args.DatasetNodeId,
		"host":               args.Host,
		"organizationName":   args.OrganizationName,
		"organizationNodeId": args.OrganizationNodeId,
		"previousOwnerName":  args.PreviousOwnerName,
	})
}

// AddedToTeamArgs are the variables for the "added-to-team" template.
type AddedToTeamArgs struct {
	Administrator      string
	TeamName           string
	Host               string
	OrganizationNodeId string
}

// AddedToTeam notifies a user that they were added to a team.
func AddedToTeam(to To, args AddedToTeamArgs) EmailRequest {
	return Message("added-to-team", to, map[string]any{
		"administrator":      args.Administrator,
		"teamName":           args.TeamName,
		"host":               args.Host,
		"organizationNodeId": args.OrganizationNodeId,
	})
}

// RehydrationCompleteArgs are the variables for the "rehydration-complete"
// template. (Note: this template uses PascalCase variable names.)
type RehydrationCompleteArgs struct {
	DatasetID           string
	DatasetVersionID    string
	RehydrationLocation string
	AWSRegion           string
}

// RehydrationComplete notifies a user that a dataset rehydration finished.
func RehydrationComplete(to To, args RehydrationCompleteArgs) EmailRequest {
	return Message("rehydration-complete", to, map[string]any{
		"DatasetID":           args.DatasetID,
		"DatasetVersionID":    args.DatasetVersionID,
		"RehydrationLocation": args.RehydrationLocation,
		"AWSRegion":           args.AWSRegion,
	})
}

// DatasetProposalSubmittedArgs are the variables for the
// "dataset-proposal-submitted" template.
type DatasetProposalSubmittedArgs struct {
	AuthorName      string
	AuthorEmail     string
	ProposalTitle   string
	WorkspaceName   string
	WorkspaceNodeId string
	AppURL          string
}

// DatasetProposalSubmitted notifies reviewers that a dataset proposal was
// submitted to a workspace.
func DatasetProposalSubmitted(to To, args DatasetProposalSubmittedArgs) EmailRequest {
	return Message("dataset-proposal-submitted", to, map[string]any{
		"AuthorName":      args.AuthorName,
		"AuthorEmail":     args.AuthorEmail,
		"ProposalTitle":   args.ProposalTitle,
		"WorkspaceName":   args.WorkspaceName,
		"WorkspaceNodeId": args.WorkspaceNodeId,
		"AppURL":          args.AppURL,
	})
}
