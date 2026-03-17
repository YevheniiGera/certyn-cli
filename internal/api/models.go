package api

import (
	"encoding/json"
	"time"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type CreateCiRunRequest struct {
	ProjectSlug    string   `json:"projectSlug"`
	EnvironmentKey string   `json:"environmentKey,omitempty"`
	ProcessSlug    string   `json:"processSlug,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Repository     string   `json:"repository,omitempty"`
	Ref            string   `json:"ref,omitempty"`
	CommitSHA      string   `json:"commitSha,omitempty"`
	Event          string   `json:"event,omitempty"`
	ExternalURL    string   `json:"externalUrl,omitempty"`
}

type CreateCiRunResponse struct {
	RunID         string `json:"runId"`
	TestCaseCount int    `json:"testCaseCount"`
	StatusPath    string `json:"statusPath"`
	CancelPath    string `json:"cancelPath"`
	StatusURL     string `json:"statusUrl"`
	AppURL        string `json:"appUrl"`
	Message       string `json:"message"`
}

type CiRunStatusResponse struct {
	RunID             string   `json:"runId"`
	State             string   `json:"state"`
	Conclusion        string   `json:"conclusion"`
	Total             int      `json:"total"`
	Passed            int      `json:"passed"`
	Failed            int      `json:"failed"`
	Blocked           int      `json:"blocked"`
	Pending           int      `json:"pending"`
	Aborted           int      `json:"aborted"`
	StartedAt         *string  `json:"startedAt"`
	CompletedAt       *string  `json:"completedAt"`
	EnvironmentKey    string   `json:"environmentKey"`
	ProcessSlug       string   `json:"processSlug"`
	Tags              []string `json:"tags"`
	CommitSHA         string   `json:"commitSha"`
	ExternalURL       string   `json:"externalUrl"`
	AppURL            string   `json:"appUrl"`
	RetryAfterSeconds int      `json:"retryAfterSeconds"`
}

type ProcessRunDetail struct {
	ID              string                 `json:"id"`
	Number          int                    `json:"number"`
	ProcessID       *string                `json:"processId"`
	ProcessName     *string                `json:"processName"`
	ProcessSlug     *string                `json:"processSlug"`
	Status          string                 `json:"status"`
	TriggerType     string                 `json:"triggerType"`
	StartedAt       *time.Time             `json:"startedAt"`
	CompletedAt     *time.Time             `json:"completedAt"`
	SummaryMarkdown *string                `json:"summaryMarkdown"`
	TotalItems      int                    `json:"totalItems"`
	PassedItems     int                    `json:"passedItems"`
	FailedItems     int                    `json:"failedItems"`
	BlockedItems    int                    `json:"blockedItems"`
	RunningItems    int                    `json:"runningItems"`
	PendingItems    int                    `json:"pendingItems"`
	Items           []ProcessRunDetailItem `json:"items"`
}

type ProcessRunDetailItem struct {
	ID              string     `json:"id"`
	TestCaseID      *string    `json:"testCaseId"`
	TestCaseName    *string    `json:"testCaseName"`
	TestCaseNumber  *int       `json:"testCaseNumber"`
	ExecutionID     *string    `json:"executionId"`
	AgentSessionID  *string    `json:"agentSessionId"`
	Status          *string    `json:"status"`
	TestOutcome     *string    `json:"testOutcome"`
	SummaryMarkdown *string    `json:"summaryMarkdown"`
	StartedAt       *time.Time `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt"`
}

type CancelCiRunRequest struct {
	Reason string `json:"reason,omitempty"`
}

type ListCiRunsResponse struct {
	Items      []CiRunListItem `json:"items"`
	TotalCount int             `json:"totalCount"`
}

type CiRunListItem struct {
	RunID          string   `json:"runId"`
	RunNumber      int      `json:"runNumber"`
	State          string   `json:"state"`
	Conclusion     string   `json:"conclusion"`
	ProcessName    string   `json:"processName"`
	ProcessSlug    string   `json:"processSlug"`
	EnvironmentKey string   `json:"environmentKey"`
	Repository     string   `json:"repository"`
	Ref            string   `json:"ref"`
	CommitSHA      string   `json:"commitSha"`
	Event          string   `json:"event"`
	ExternalURL    string   `json:"externalUrl"`
	Tags           []string `json:"tags"`
	Total          int      `json:"total"`
	Passed         int      `json:"passed"`
	Failed         int      `json:"failed"`
	Blocked        int      `json:"blocked"`
	Pending        int      `json:"pending"`
}

type RunnerPool struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	IsActive           bool   `json:"isActive"`
	PoolKind           string `json:"poolKind"`
	IsShared           bool   `json:"isShared"`
	CloudRegion        string `json:"cloudRegion"`
	ScaleSetResourceID string `json:"scaleSetResourceId"`
	MinRunners         int    `json:"minRunners"`
	MaxRunners         int    `json:"maxRunners"`
	SlotsPerRunner     int    `json:"slotsPerRunner"`
}

type CreateRunnerPoolRequest struct {
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	PoolKind           string `json:"poolKind,omitempty"`
	IsShared           bool   `json:"isShared,omitempty"`
	CloudRegion        string `json:"cloudRegion,omitempty"`
	ScaleSetResourceID string `json:"scaleSetResourceId,omitempty"`
	MinRunners         *int   `json:"minRunners,omitempty"`
	MaxRunners         *int   `json:"maxRunners,omitempty"`
	SlotsPerRunner     *int   `json:"slotsPerRunner,omitempty"`
}

type CreateRunnerTokenRequest struct {
	TokenMode string `json:"tokenMode,omitempty"`
	MaxUses   *int   `json:"maxUses,omitempty"`
}

type RunnerTokenResponse struct {
	PoolID       string    `json:"poolId"`
	Token        string    `json:"token"`
	ExpiresAtUTC time.Time `json:"expiresAtUtc"`
	TokenMode    string    `json:"tokenMode"`
	MaxUses      *int      `json:"maxUses"`
}

type SelfHostedRunner struct {
	ID             string    `json:"id"`
	PoolID         string    `json:"poolId"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	MaxConcurrency int       `json:"maxConcurrency"`
	AvailableSlots int       `json:"availableSlots"`
	LastHeartbeat  time.Time `json:"lastHeartbeatUtc"`
}

type ProjectsOverviewResponse struct {
	DefaultProjectID string                `json:"defaultProjectId"`
	Projects         []ProjectOverviewItem `json:"projects"`
}

type ProjectOverviewItem struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type ProcessDefinition struct {
	ID            string               `json:"id"`
	ProjectID     string               `json:"projectId"`
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	IsExploratory bool                 `json:"isExploratory"`
	IsActive      bool                 `json:"isActive"`
	Configuration ProcessConfiguration `json:"configuration"`
}

type ProcessConfiguration struct {
	TicketLabels []string `json:"ticketLabels"`
	TestCaseTags []string `json:"testCaseTags"`
}

type PagedResponse[T any] struct {
	Items       []T  `json:"items"`
	TotalCount  int  `json:"totalCount"`
	Page        int  `json:"page"`
	PageSize    int  `json:"pageSize"`
	TotalPages  int  `json:"totalPages"`
	HasNextPage bool `json:"hasNextPage"`
	HasPrevPage bool `json:"hasPreviousPage"`
}

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Color       string `json:"color"`
	TenantID    string `json:"tenantId"`
}

type ProjectInput struct {
	Name        string `json:"name,omitempty"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

type CreateProjectRequest struct {
	Project ProjectInput `json:"project"`
}

type UpdateProjectRequest struct {
	ID      string       `json:"id"`
	Project ProjectInput `json:"project"`
}

type Environment struct {
	ID              string `json:"id"`
	ProjectID       string `json:"projectId"`
	Key             string `json:"key"`
	Label           string `json:"label"`
	BaseURL         string `json:"baseUrl"`
	IsDefault       bool   `json:"isDefault"`
	Version         string `json:"version"`
	AnthropicModel  string `json:"anthropicModel"`
	ExecutionTarget string `json:"executionTarget"`
	RunnerPoolID    string `json:"runnerPoolId"`
}

type EnvironmentInput struct {
	Key             string `json:"key,omitempty"`
	Label           string `json:"label,omitempty"`
	BaseURL         string `json:"baseUrl,omitempty"`
	Version         string `json:"version,omitempty"`
	Changelog       string `json:"changelog,omitempty"`
	IsDefault       *bool  `json:"isDefault,omitempty"`
	AnthropicModel  string `json:"anthropicModel,omitempty"`
	ExecutionTarget string `json:"executionTarget,omitempty"`
	RunnerPoolID    string `json:"runnerPoolId,omitempty"`
}

type EnvironmentVariable struct {
	ID            string `json:"id"`
	EnvironmentID string `json:"environmentId"`
	Name          string `json:"name"`
	Value         string `json:"value"`
	Description   string `json:"description"`
}

type EnvironmentVariableInput struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type TestCase struct {
	ID                                 string    `json:"id"`
	Number                             int       `json:"number"`
	ProjectID                          string    `json:"projectId"`
	Name                               string    `json:"name"`
	Description                        *string   `json:"description"`
	Instructions                       string    `json:"instructions"`
	Tags                               []string  `json:"tags"`
	IsQuarantined                      bool      `json:"isQuarantined"`
	NeedsReview                        bool      `json:"needsReview"`
	MinimumSupportedEnvironmentVersion *string   `json:"minimumSupportedEnvironmentVersion"`
	CreatedAt                          time.Time `json:"createdAt"`
	UpdatedAt                          time.Time `json:"updatedAt"`
}

type TestCaseInput struct {
	Name                               string   `json:"name,omitempty"`
	Description                        string   `json:"description,omitempty"`
	Instructions                       string   `json:"instructions,omitempty"`
	Tags                               []string `json:"tags,omitempty"`
	IsQuarantined                      *bool    `json:"isQuarantined,omitempty"`
	NeedsReview                        *bool    `json:"needsReview,omitempty"`
	CreatedByAgentTemplateID           string   `json:"createdByAgentTemplateId,omitempty"`
	MinimumSupportedEnvironmentVersion string   `json:"minimumSupportedEnvironmentVersion,omitempty"`
	CreatedDuringExecutionID           string   `json:"createdDuringExecutionId,omitempty"`
}

type UpdateTestCaseTagsRequest struct {
	Tags string `json:"tags"`
}

type ExecuteTestCaseResponse struct {
	TestCaseID  string `json:"testCaseId"`
	ExecutionID string `json:"executionId"`
	Status      string `json:"status"`
}

type ExecuteTestCasesBulkRequest struct {
	TestCaseIDs []string `json:"testCaseIds"`
}

type ExecuteTestCasesBulkResponse struct {
	SuccessCount int                          `json:"successCount"`
	FailureCount int                          `json:"failureCount"`
	Executions   []ExecuteTestCaseBulkOutcome `json:"executions"`
}

type ExecuteTestCaseBulkOutcome struct {
	TestCaseID  string  `json:"testCaseId"`
	ExecutionID *string `json:"executionId"`
	Status      string  `json:"status"`
}

type TestCaseOverview struct {
	ID                     string     `json:"id"`
	Number                 int        `json:"number"`
	ProjectID              string     `json:"projectId"`
	Name                   string     `json:"name"`
	Description            *string    `json:"description"`
	Tags                   []string   `json:"tags"`
	IsQuarantined          bool       `json:"isQuarantined"`
	NeedsReview            bool       `json:"needsReview"`
	TotalRuns              int        `json:"totalRuns"`
	PassRate               float64    `json:"passRate"`
	FlakinessRate          float64    `json:"flakinessRate"`
	LastRunStatus          *string    `json:"lastRunStatus"`
	LastRunStartedAt       *time.Time `json:"lastRunStartedAt"`
	ActivityStatus         *string    `json:"activityStatus"`
	ActiveTickets          int        `json:"activeTickets"`
	CriticalOrMajorTickets int        `json:"criticalOrMajorTickets"`
	ActiveTicketID         *string    `json:"activeTicketId"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type TestCaseFlakiness struct {
	TestCaseID       string     `json:"testCaseId"`
	FlakinessScore   float64    `json:"flakinessScore"`
	TotalRuns        int        `json:"totalRuns"`
	PassedRuns       int        `json:"passedRuns"`
	FailedRuns       int        `json:"failedRuns"`
	FlipCount        int        `json:"flipCount"`
	IsQuarantined    bool       `json:"isQuarantined"`
	ShouldQuarantine bool       `json:"shouldQuarantine"`
	LastRunAt        *time.Time `json:"lastRunAt"`
}

type TestCaseReport struct {
	TestCase              TestCaseReportSummary         `json:"testCase"`
	LastRuns              []TestCaseRun                 `json:"lastRuns"`
	PassRate              float64                       `json:"passRate"`
	FlakinessRate         float64                       `json:"flakinessRate"`
	TotalRuns             int                           `json:"totalRuns"`
	PassedRuns            int                           `json:"passedRuns"`
	FailedRuns            int                           `json:"failedRuns"`
	ActiveTickets         int                           `json:"activeTickets"`
	CriticalTickets       int                           `json:"criticalTickets"`
	MajorTickets          int                           `json:"majorTickets"`
	MedianDurationSeconds float64                       `json:"medianDurationSeconds"`
	P95DurationSeconds    float64                       `json:"p95DurationSeconds"`
	Environments          []TestCaseEnvironmentSummary  `json:"environments"`
	ActiveTicketList      []TestCaseReportTicketSummary `json:"activeTicketList"`
}

type TestCaseReportSummary struct {
	ID           string  `json:"id"`
	Number       int     `json:"number"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Instructions string  `json:"instructions"`
	Tags         string  `json:"tags"`
	ProjectID    string  `json:"projectId"`
}

type TestCaseRun struct {
	ID              string     `json:"id"`
	RunID           string     `json:"runId"`
	TicketID        *string    `json:"ticketId"`
	Status          string     `json:"status"`
	DurationSeconds *int       `json:"durationSeconds"`
	StartedAt       *time.Time `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt"`
	EnvironmentKey  *string    `json:"environmentKey"`
	TriggerType     *string    `json:"triggerType"`
	OpenTickets     int        `json:"openTickets"`
	SummaryMarkdown *string    `json:"summaryMarkdown"`
}

type TestCaseEnvironmentSummary struct {
	Key    string `json:"key"`
	Total  int    `json:"total"`
	Passed int    `json:"passed"`
	Failed int    `json:"failed"`
}

type TestCaseReportTicketSummary struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"`
	Status     string    `json:"status"`
	FoundAt    time.Time `json:"foundAt"`
	AgentRunID *string   `json:"agentRunId"`
	RunID      *string   `json:"runId"`
}

type ListIssuesParams struct {
	Activity           string
	Type               string
	Severity           string
	Status             string
	AgentID            string
	EnvironmentKey     string
	EnvironmentVersion string
	Page               int
	PageSize           int
}

type Issue struct {
	ID                  string            `json:"id"`
	Number              int               `json:"number"`
	ProjectID           string            `json:"projectId"`
	Title               string            `json:"title"`
	Description         *string           `json:"description"`
	Type                string            `json:"type"`
	Severity            string            `json:"severity"`
	Status              string            `json:"status"`
	Labels              []string          `json:"labels"`
	NeedsReview         bool              `json:"needsReview"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
	RunActivity         *string           `json:"runActivity"`
	HasActiveExecution  bool              `json:"hasActiveExecution"`
	LastExecutionID     *string           `json:"lastExecutionId"`
	LastExecutionStatus *string           `json:"lastExecutionStatus"`
	LastOutcome         *string           `json:"lastOutcome"`
	LastRunStartedAt    *time.Time        `json:"lastRunStartedAt"`
	LastRunCompletedAt  *time.Time        `json:"lastRunCompletedAt"`
	TestCaseID          *string           `json:"testCaseId"`
	TestCase            *IssueTestCaseRef `json:"testCase"`
	EnvironmentVersion  *string           `json:"environmentVersion"`
}

type IssueInput struct {
	Title                    string   `json:"title,omitempty"`
	Description              string   `json:"description,omitempty"`
	Type                     string   `json:"type,omitempty"`
	Severity                 string   `json:"severity,omitempty"`
	Labels                   []string `json:"labels,omitempty"`
	Status                   string   `json:"status,omitempty"`
	NeedsReview              *bool    `json:"needsReview,omitempty"`
	CreatedByAgentTemplateID string   `json:"createdByAgentTemplateId,omitempty"`
	CreatedDuringExecutionID string   `json:"createdDuringExecutionId,omitempty"`
}

type UpdateIssueLabelsRequest struct {
	Labels []string `json:"labels"`
}

type LinkIssueTestCaseInput struct {
	TestCaseID string `json:"testCaseId"`
	Priority   string `json:"priority,omitempty"`
}

type LinkIssueTestCaseResponse struct {
	ID string `json:"id"`
}

type IssueCommentInput struct {
	Body string `json:"body"`
}

type IssueComment struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type IssueAttachmentInput struct {
	Kind        string `json:"kind"`
	Base64Data  string `json:"base64Data,omitempty"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

type IssueAttachment struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	URL         *string   `json:"url"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type IssueVerificationInput struct {
	Decision         string `json:"decision"`
	Summary          string `json:"summary"`
	Expected         string `json:"expected,omitempty"`
	Observed         string `json:"observed,omitempty"`
	Base64Screenshot string `json:"base64Screenshot,omitempty"`
}

type IssueVerification struct {
	ID               string    `json:"id"`
	Decision         string    `json:"decision"`
	Summary          string    `json:"summary"`
	Expected         *string   `json:"expected"`
	Observed         *string   `json:"observed"`
	Base64Screenshot *string   `json:"base64Screenshot"`
	VerifiedAt       time.Time `json:"verifiedAt"`
}

type IssueTestCaseRef struct {
	ID       string  `json:"id"`
	Number   int     `json:"number"`
	Name     string  `json:"name"`
	Severity string  `json:"severity"`
	Notes    *string `json:"notes"`
}

type IssueOverview struct {
	ID                       string     `json:"id"`
	Number                   int        `json:"number"`
	ProjectID                string     `json:"projectId"`
	Title                    string     `json:"title"`
	Description              *string    `json:"description"`
	Status                   string     `json:"status"`
	RunActivity              string     `json:"runActivity"`
	Severity                 string     `json:"severity"`
	Type                     string     `json:"type"`
	Labels                   []string   `json:"labels"`
	NeedsReview              bool       `json:"needsReview"`
	Source                   string     `json:"source"`
	CreatedByAgentID         *string    `json:"createdByAgentId"`
	CreatedByAgentName       *string    `json:"createdByAgentName"`
	CreatedDuringExecutionID *string    `json:"createdDuringExecutionId"`
	AffectedTestCaseIDs      []string   `json:"affectedTestCaseIds"`
	AffectedTestCaseCount    int        `json:"affectedTestCaseCount"`
	VerificationCount        int        `json:"verificationCount"`
	LastVerifiedAt           *time.Time `json:"lastVerifiedAt"`
	LastVerificationStatus   *string    `json:"lastVerificationStatus"`
	CommentCount             int        `json:"commentCount"`
	AttachmentCount          int        `json:"attachmentCount"`
	EnvironmentKey           *string    `json:"environmentKey"`
	EnvironmentID            *string    `json:"environmentId"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
	AgeInDays                int        `json:"ageInDays"`
	LastActivityAt           time.Time  `json:"lastActivityAt"`
}

type RetestIssueResponse struct {
	Success bool    `json:"success"`
	Message *string `json:"message"`
}

type ListObservationsParams struct {
	Status                         string
	DiscoveredInEnvironmentKey     string
	DiscoveredInEnvironmentVersion string
	ProcessRunID                   string
	CreatedByAgentTemplateID       string
	CreatedAfter                   string
	Page                           int
	PageSize                       int
}

type ObservationAttachment struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	URL         *string   `json:"url"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Observation struct {
	ID                             string                  `json:"id"`
	Number                         int                     `json:"number"`
	ProjectID                      string                  `json:"projectId"`
	Title                          string                  `json:"title"`
	Description                    *string                 `json:"description"`
	Status                         string                  `json:"status"`
	DiscoveredInEnvironmentKey     *string                 `json:"discoveredInEnvironmentKey"`
	DiscoveredInEnvironmentVersion *string                 `json:"discoveredInEnvironmentVersion"`
	CreatedDuringExecutionID       *string                 `json:"createdDuringExecutionId"`
	DiscoveredDuringRunID          *string                 `json:"discoveredDuringRunId"`
	CreatedByAgentTemplateID       *string                 `json:"createdByAgentTemplateId"`
	CreatedByUserID                *string                 `json:"createdByUserId"`
	AttachmentCount                int                     `json:"attachmentCount"`
	Source                         string                  `json:"source"`
	CreatedAt                      time.Time               `json:"createdAt"`
	UpdatedAt                      time.Time               `json:"updatedAt"`
	Attachments                    []ObservationAttachment `json:"attachments"`
}

type ObservationInput struct {
	Title                          string `json:"title,omitempty"`
	Description                    string `json:"description,omitempty"`
	Status                         string `json:"status,omitempty"`
	DiscoveredInEnvironmentKey     string `json:"discoveredInEnvironmentKey,omitempty"`
	DiscoveredInEnvironmentVersion string `json:"discoveredInEnvironmentVersion,omitempty"`
	CreatedDuringExecutionID       string `json:"createdDuringExecutionId,omitempty"`
	DiscoveredDuringRunID          string `json:"discoveredDuringRunId,omitempty"`
}

type ObservationSearchHit struct {
	ID                             string    `json:"id"`
	Number                         int       `json:"number"`
	Title                          string    `json:"title"`
	Description                    *string   `json:"description"`
	Status                         string    `json:"status"`
	DiscoveredInEnvironmentKey     *string   `json:"discoveredInEnvironmentKey"`
	DiscoveredInEnvironmentVersion *string   `json:"discoveredInEnvironmentVersion"`
	CreatedAt                      time.Time `json:"createdAt"`
	RelevanceScore                 *float64  `json:"relevanceScore"`
}

type SearchObservationsResponse struct {
	Success      bool                   `json:"success"`
	Error        *string                `json:"error"`
	Observations []ObservationSearchHit `json:"observations"`
	TotalCount   int                    `json:"totalCount"`
}

type PromoteObservationToTicketInput struct {
	Type     string   `json:"type"`
	Severity string   `json:"severity,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

type PromoteObservationToWikiInput struct {
	Section              string `json:"section"`
	Mode                 string `json:"mode"`
	ContentMarkdown      string `json:"contentMarkdown"`
	ExpectedExistingText string `json:"expectedExistingText,omitempty"`
}

type ListExecutionsParams struct {
	Status         string
	AgentID        string
	EnvironmentKey string
	Page           int
	PageSize       int
}

type ListConversationEventsParams struct {
	Before   *time.Time
	Page     int
	PageSize int
}

type ExecutionListItem struct {
	ID                   string              `json:"id"`
	TicketID             string              `json:"ticketId"`
	Status               string              `json:"status"`
	TriggerType          string              `json:"triggerType"`
	StartedAt            *time.Time          `json:"startedAt"`
	CompletedAt          *time.Time          `json:"completedAt"`
	CreatedAt            time.Time           `json:"createdAt"`
	Duration             int                 `json:"duration"`
	AgentTemplateID      string              `json:"agentTemplateId"`
	TestCasesCount       int                 `json:"testCasesCount"`
	FindingsCount        int                 `json:"findingsCount"`
	Ticket               *ExecutionTicketRef `json:"ticket"`
	Agent                *ExecutionAgentRef  `json:"agent"`
	EnvironmentKey       *string             `json:"environmentKey"`
	AgentSessionID       *string             `json:"agentSessionId"`
	SummaryMarkdown      *string             `json:"summaryMarkdown"`
	InstructionsMarkdown *string             `json:"instructionsMarkdown"`
	DisplayName          string              `json:"displayName"`
	EffectiveTestOutcome string              `json:"effectiveTestOutcome"`
	ExecutionTarget      *string             `json:"executionTarget"`
}

type ExecutionTicketRef struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Type       string  `json:"type"`
	Severity   string  `json:"severity"`
	Status     string  `json:"status"`
	TestCaseID *string `json:"testCaseId"`
}

type ExecutionAgentRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Execution struct {
	ID                  string              `json:"id"`
	TicketID            string              `json:"ticketId"`
	OriginalExecutionID *string             `json:"originalExecutionId"`
	AgentSessionID      *string             `json:"agentSessionId"`
	Status              string              `json:"status"`
	TriggerType         string              `json:"triggerType"`
	StartedAt           *time.Time          `json:"startedAt"`
	CompletedAt         *time.Time          `json:"completedAt"`
	TestCases           []ExecutionTestCase `json:"testCases"`
	Notes               []ExecutionNote     `json:"notes"`
	Artifacts           []ExecutionArtifact `json:"artifacts"`
	SummaryMarkdown     *string             `json:"summaryMarkdown"`
}

type ExecutionArtifact struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	URI         *string   `json:"uri"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ExecutionNote struct {
	ID        string    `json:"id"`
	Markdown  *string   `json:"markdown"`
	CreatedAt time.Time `json:"createdAt"`
}

type ExecutionTestCase struct {
	ID                   string     `json:"id"`
	TestCaseID           string     `json:"testCaseId"`
	Status               string     `json:"status"`
	Severity             string     `json:"severity"`
	ResultMarkdown       *string    `json:"resultMarkdown"`
	StartedAt            *time.Time `json:"startedAt"`
	CompletedAt          *time.Time `json:"completedAt"`
	AddedDuringExecution bool       `json:"addedDuringExecution"`
	EnvironmentVersion   *string    `json:"environmentVersion"`
}

type ConversationEvent struct {
	Type         string          `json:"type"`
	TimestampUTC time.Time       `json:"timestampUtc"`
	Payload      json.RawMessage `json:"payload"`
}

type AskAdvisorRequest struct {
	Message             string `json:"message"`
	ProjectID           string `json:"projectId,omitempty"`
	MaxToolIterations   *int   `json:"maxToolIterations,omitempty"`
	MaxOutputTokenCount *int   `json:"maxOutputTokenCount,omitempty"`
}

type ChatResponse struct {
	ConversationID string           `json:"conversationId"`
	MessageID      string           `json:"messageId"`
	Content        string           `json:"content"`
	Role           string           `json:"role"`
	ToolCalls      []ToolCallResult `json:"toolCalls"`
	CreatedAt      time.Time        `json:"createdAt"`
}

type ToolCallResult struct {
	ToolName string `json:"toolName"`
	Success  bool   `json:"success"`
	Result   any    `json:"result"`
	Error    string `json:"error"`
}
