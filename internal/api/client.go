package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	httpClient  *http.Client
	baseURL     string
	apiKey      string
	accessToken string
	userAgent   string
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("api request failed with status %d", e.StatusCode)
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
		userAgent:  "certyn-cli/dev",
	}
}

func (c *Client) SetAccessToken(token string) {
	c.accessToken = strings.TrimSpace(token)
}

func (c *Client) SetUserAgent(ua string) {
	if strings.TrimSpace(ua) != "" {
		c.userAgent = ua
	}
}

func (c *Client) StatusURL(runID string) string {
	base := strings.TrimSuffix(c.baseURL, "/")
	return fmt.Sprintf("%s/ci/runs/%s", base, url.PathEscape(strings.TrimSpace(runID)))
}

func (c *Client) CreateCiRun(ctx context.Context, request CreateCiRunRequest, idempotencyKey string) (*CreateCiRunResponse, error) {
	var resp CreateCiRunResponse
	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	if err := c.doJSON(ctx, http.MethodPost, "/ci/runs", request, &resp, headers); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetCiRunStatus(ctx context.Context, runID string) (*CiRunStatusResponse, error) {
	var resp CiRunStatusResponse
	if err := c.doJSON(ctx, http.MethodGet, "/ci/runs/"+url.PathEscape(runID), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetProcessRun(ctx context.Context, projectID, processRunID string) (*ProcessRunDetail, error) {
	endpoint := fmt.Sprintf("/projects/%s/process-runs/%s", url.PathEscape(projectID), url.PathEscape(processRunID))
	var resp ProcessRunDetail
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CancelCiRun(ctx context.Context, runID, reason string) error {
	body := CancelCiRunRequest{Reason: reason}
	return c.doJSON(ctx, http.MethodPost, "/ci/runs/"+url.PathEscape(runID)+"/cancel", body, nil, nil)
}

func (c *Client) ListCiRuns(ctx context.Context, projectID string, take, skip int) (*ListCiRunsResponse, error) {
	endpoint := fmt.Sprintf("/projects/%s/ci-runs?take=%d&skip=%d", url.PathEscape(projectID), take, skip)
	var resp ListCiRunsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetProjectsOverview(ctx context.Context) (*ProjectsOverviewResponse, error) {
	var resp ProjectsOverviewResponse
	if err := c.doJSON(ctx, http.MethodGet, "/projects/overview", nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListProjects(ctx context.Context, page, pageSize int) (*PagedResponse[Project], error) {
	endpoint := fmt.Sprintf("/projects?page=%d&pageSize=%d", page, pageSize)
	var resp PagedResponse[Project]
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var resp Project
	if err := c.doJSON(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListProcesses(ctx context.Context, projectID string) ([]ProcessDefinition, error) {
	endpoint := fmt.Sprintf("/projects/%s/processes", url.PathEscape(projectID))
	var resp []ProcessDefinition
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) CreateProject(ctx context.Context, input ProjectInput) (*Project, error) {
	var resp Project
	if err := c.doJSON(ctx, http.MethodPost, "/projects", CreateProjectRequest{Project: input}, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateProject(ctx context.Context, id string, input ProjectInput) error {
	return c.doJSON(ctx, http.MethodPut, "/projects/"+url.PathEscape(id), UpdateProjectRequest{ID: id, Project: input}, nil, nil)
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/projects/"+url.PathEscape(id), nil, nil, nil)
}

func (c *Client) ListEnvironments(ctx context.Context, projectID string, page, pageSize int) (*PagedResponse[Environment], error) {
	endpoint := fmt.Sprintf("/projects/%s/environments?page=%d&pageSize=%d", url.PathEscape(projectID), page, pageSize)
	var resp PagedResponse[Environment]
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetEnvironment(ctx context.Context, projectID, environmentID string) (*Environment, error) {
	endpoint := fmt.Sprintf("/projects/%s/environments/%s", url.PathEscape(projectID), url.PathEscape(environmentID))
	var resp Environment
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateEnvironment(ctx context.Context, projectID string, input EnvironmentInput) (*Environment, error) {
	endpoint := fmt.Sprintf("/projects/%s/environments", url.PathEscape(projectID))
	var resp Environment
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, projectID, environmentID string, input EnvironmentInput) error {
	endpoint := fmt.Sprintf("/projects/%s/environments/%s", url.PathEscape(projectID), url.PathEscape(environmentID))
	return c.doJSON(ctx, http.MethodPut, endpoint, input, nil, nil)
}

func (c *Client) DeleteEnvironment(ctx context.Context, projectID, environmentID string) error {
	endpoint := fmt.Sprintf("/projects/%s/environments/%s", url.PathEscape(projectID), url.PathEscape(environmentID))
	return c.doJSON(ctx, http.MethodDelete, endpoint, nil, nil, nil)
}

func (c *Client) ListEnvironmentVariables(ctx context.Context, projectID, environmentID string) ([]EnvironmentVariable, error) {
	endpoint := fmt.Sprintf("/projects/%s/environments/%s/variables", url.PathEscape(projectID), url.PathEscape(environmentID))
	var resp []EnvironmentVariable
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) CreateEnvironmentVariable(ctx context.Context, projectID, environmentID string, input EnvironmentVariableInput) (*EnvironmentVariable, error) {
	endpoint := fmt.Sprintf("/projects/%s/environments/%s/variables", url.PathEscape(projectID), url.PathEscape(environmentID))
	var resp EnvironmentVariable
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateEnvironmentVariable(ctx context.Context, projectID, environmentID, variableID string, input EnvironmentVariableInput) error {
	endpoint := fmt.Sprintf("/projects/%s/environments/%s/variables/%s", url.PathEscape(projectID), url.PathEscape(environmentID), url.PathEscape(variableID))
	return c.doJSON(ctx, http.MethodPut, endpoint, input, nil, nil)
}

func (c *Client) DeleteEnvironmentVariable(ctx context.Context, projectID, environmentID, variableID string) error {
	endpoint := fmt.Sprintf("/projects/%s/environments/%s/variables/%s", url.PathEscape(projectID), url.PathEscape(environmentID), url.PathEscape(variableID))
	return c.doJSON(ctx, http.MethodDelete, endpoint, nil, nil, nil)
}

func (c *Client) ListRunnerPools(ctx context.Context) ([]RunnerPool, error) {
	var resp []RunnerPool
	if err := c.doJSON(ctx, http.MethodGet, "/self-hosted/pools", nil, &resp, nil); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) CreateRunnerPool(ctx context.Context, input CreateRunnerPoolRequest) (*RunnerPool, error) {
	var resp RunnerPool
	if err := c.doJSON(ctx, http.MethodPost, "/self-hosted/pools", input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteRunnerPool(ctx context.Context, poolID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/self-hosted/pools/"+url.PathEscape(poolID), nil, nil, nil)
}

func (c *Client) CreateRunnerToken(ctx context.Context, poolID string, input CreateRunnerTokenRequest) (*RunnerTokenResponse, error) {
	var resp RunnerTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, "/self-hosted/pools/"+url.PathEscape(poolID)+"/registration-tokens", input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListRunners(ctx context.Context) ([]SelfHostedRunner, error) {
	var resp []SelfHostedRunner
	if err := c.doJSON(ctx, http.MethodGet, "/self-hosted/runners", nil, &resp, nil); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) DrainRunner(ctx context.Context, runnerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/self-hosted/runners/"+url.PathEscape(runnerID)+"/drain", nil, nil, nil)
}

func (c *Client) ResumeRunner(ctx context.Context, runnerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/self-hosted/runners/"+url.PathEscape(runnerID)+"/resume", nil, nil, nil)
}

func (c *Client) ListTestCases(ctx context.Context, projectID string, isQuarantined *bool, tag string, page, pageSize int) (*PagedResponse[TestCase], error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases", url.PathEscape(projectID))
	query := url.Values{}
	if isQuarantined != nil {
		query.Set("isQuarantined", strconv.FormatBool(*isQuarantined))
	}
	if strings.TrimSpace(tag) != "" {
		query.Set("tag", strings.TrimSpace(tag))
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))

	var resp PagedResponse[TestCase]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTestCase(ctx context.Context, projectID, testCaseID string) (*TestCase, error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases/%s", url.PathEscape(projectID), url.PathEscape(testCaseID))
	var resp TestCase
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateTestCase(ctx context.Context, projectID string, input TestCaseInput) (*TestCase, error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases", url.PathEscape(projectID))
	var resp TestCase
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateTestCase(ctx context.Context, projectID, testCaseID string, input TestCaseInput) (*TestCase, error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases/%s", url.PathEscape(projectID), url.PathEscape(testCaseID))
	var resp TestCase
	if err := c.doJSON(ctx, http.MethodPut, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteTestCase(ctx context.Context, projectID, testCaseID string) error {
	endpoint := fmt.Sprintf("/projects/%s/testcases/%s", url.PathEscape(projectID), url.PathEscape(testCaseID))
	return c.doJSON(ctx, http.MethodDelete, endpoint, nil, nil, nil)
}

func (c *Client) UpdateTestCaseTags(ctx context.Context, projectID, testCaseID, tags string) error {
	endpoint := fmt.Sprintf("/projects/%s/testcases/%s/tags", url.PathEscape(projectID), url.PathEscape(testCaseID))
	return c.doJSON(ctx, http.MethodPatch, endpoint, UpdateTestCaseTagsRequest{Tags: tags}, nil, nil)
}

func (c *Client) ExecuteTestCase(ctx context.Context, projectID, testCaseID string) (*ExecuteTestCaseResponse, error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases/%s/execute", url.PathEscape(projectID), url.PathEscape(testCaseID))
	var resp ExecuteTestCaseResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ExecuteTestCasesBulk(ctx context.Context, projectID string, testCaseIDs []string) (*ExecuteTestCasesBulkResponse, error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases/execute", url.PathEscape(projectID))
	var resp ExecuteTestCasesBulkResponse
	// This endpoint expects a raw JSON string array body.
	if err := c.doJSON(ctx, http.MethodPost, endpoint, testCaseIDs, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListTestCaseOverview(ctx context.Context, projectID string, page, pageSize int) (*PagedResponse[TestCaseOverview], error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases/overview", url.PathEscape(projectID))
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))

	var resp PagedResponse[TestCaseOverview]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTestCaseReport(ctx context.Context, projectID, testCaseID string) (*TestCaseReport, error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases/%s/report", url.PathEscape(projectID), url.PathEscape(testCaseID))
	var resp TestCaseReport
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTestCaseFlakiness(ctx context.Context, projectID, testCaseID string) (*TestCaseFlakiness, error) {
	endpoint := fmt.Sprintf("/projects/%s/testcases/%s/flakiness", url.PathEscape(projectID), url.PathEscape(testCaseID))
	var resp TestCaseFlakiness
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListFlakyTestCases(ctx context.Context, projectID string, threshold float64) ([]TestCaseFlakiness, error) {
	endpoint := fmt.Sprintf("/projects/%s/flaky-testcases", url.PathEscape(projectID))
	query := url.Values{}
	query.Set("threshold", strconv.FormatFloat(threshold, 'f', -1, 64))

	var resp []TestCaseFlakiness
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) ListIssues(ctx context.Context, projectID string, params ListIssuesParams) (*PagedResponse[Issue], error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets", url.PathEscape(projectID))
	query := url.Values{}
	if strings.TrimSpace(params.Activity) != "" {
		query.Set("activity", strings.TrimSpace(params.Activity))
	}
	if strings.TrimSpace(params.Type) != "" {
		query.Set("type", strings.TrimSpace(params.Type))
	}
	if strings.TrimSpace(params.Severity) != "" {
		query.Set("severity", strings.TrimSpace(params.Severity))
	}
	if strings.TrimSpace(params.Status) != "" {
		query.Set("status", strings.TrimSpace(params.Status))
	}
	if strings.TrimSpace(params.AgentID) != "" {
		query.Set("agentId", strings.TrimSpace(params.AgentID))
	}
	if strings.TrimSpace(params.EnvironmentKey) != "" {
		query.Set("environmentKey", strings.TrimSpace(params.EnvironmentKey))
	}
	if strings.TrimSpace(params.EnvironmentVersion) != "" {
		query.Set("environmentVersion", strings.TrimSpace(params.EnvironmentVersion))
	}
	query.Set("page", strconv.Itoa(params.Page))
	query.Set("pageSize", strconv.Itoa(params.PageSize))

	var resp PagedResponse[Issue]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetIssue(ctx context.Context, projectID, issueID string) (*Issue, error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s", url.PathEscape(projectID), url.PathEscape(issueID))
	var resp Issue
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateIssue(ctx context.Context, projectID string, input IssueInput) (*Issue, error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets", url.PathEscape(projectID))
	var resp Issue
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateIssue(ctx context.Context, projectID, issueID string, input IssueInput) error {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s", url.PathEscape(projectID), url.PathEscape(issueID))
	return c.doJSON(ctx, http.MethodPut, endpoint, input, nil, nil)
}

func (c *Client) DeleteIssue(ctx context.Context, projectID, issueID string) error {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s", url.PathEscape(projectID), url.PathEscape(issueID))
	return c.doJSON(ctx, http.MethodDelete, endpoint, nil, nil, nil)
}

func (c *Client) UpdateIssueLabels(ctx context.Context, projectID, issueID string, labels []string) error {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s/labels", url.PathEscape(projectID), url.PathEscape(issueID))
	return c.doJSON(ctx, http.MethodPatch, endpoint, UpdateIssueLabelsRequest{Labels: labels}, nil, nil)
}

func (c *Client) LinkIssueTestCase(ctx context.Context, projectID, issueID string, input LinkIssueTestCaseInput) (*LinkIssueTestCaseResponse, error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s/testcases", url.PathEscape(projectID), url.PathEscape(issueID))
	var resp LinkIssueTestCaseResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) AddIssueComment(ctx context.Context, projectID, issueID, body string) (*IssueComment, error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s/comments", url.PathEscape(projectID), url.PathEscape(issueID))
	var resp IssueComment
	if err := c.doJSON(ctx, http.MethodPost, endpoint, IssueCommentInput{Body: body}, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) AddIssueAttachment(ctx context.Context, projectID, issueID string, input IssueAttachmentInput) (*IssueAttachment, error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s/attachments", url.PathEscape(projectID), url.PathEscape(issueID))
	var resp IssueAttachment
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) AddIssueVerification(ctx context.Context, projectID, issueID string, input IssueVerificationInput) (*IssueVerification, error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s/verifications", url.PathEscape(projectID), url.PathEscape(issueID))
	var resp IssueVerification
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListIssuesOverview(ctx context.Context, projectID, environmentKey string, page, pageSize int) (*PagedResponse[IssueOverview], error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/overview", url.PathEscape(projectID))
	query := url.Values{}
	if strings.TrimSpace(environmentKey) != "" {
		query.Set("environmentKey", strings.TrimSpace(environmentKey))
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))

	var resp PagedResponse[IssueOverview]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RetestIssue(ctx context.Context, projectID, issueID string) (*RetestIssueResponse, error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s/retest", url.PathEscape(projectID), url.PathEscape(issueID))
	var resp RetestIssueResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListObservations(ctx context.Context, projectID string, params ListObservationsParams) (*PagedResponse[Observation], error) {
	endpoint := fmt.Sprintf("/projects/%s/observations", url.PathEscape(projectID))
	query := url.Values{}
	if strings.TrimSpace(params.Status) != "" {
		query.Set("status", strings.TrimSpace(params.Status))
	}
	if strings.TrimSpace(params.DiscoveredInEnvironmentKey) != "" {
		query.Set("discoveredInEnvironmentKey", strings.TrimSpace(params.DiscoveredInEnvironmentKey))
	}
	if strings.TrimSpace(params.DiscoveredInEnvironmentVersion) != "" {
		query.Set("discoveredInEnvironmentVersion", strings.TrimSpace(params.DiscoveredInEnvironmentVersion))
	}
	if strings.TrimSpace(params.ProcessRunID) != "" {
		query.Set("processRunId", strings.TrimSpace(params.ProcessRunID))
	}
	if strings.TrimSpace(params.CreatedByAgentTemplateID) != "" {
		query.Set("createdByAgentTemplateId", strings.TrimSpace(params.CreatedByAgentTemplateID))
	}
	if strings.TrimSpace(params.CreatedAfter) != "" {
		query.Set("createdAfter", strings.TrimSpace(params.CreatedAfter))
	}
	query.Set("page", strconv.Itoa(params.Page))
	query.Set("pageSize", strconv.Itoa(params.PageSize))

	var resp PagedResponse[Observation]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetObservation(ctx context.Context, projectID, observationID string) (*Observation, error) {
	endpoint := fmt.Sprintf("/projects/%s/observations/%s", url.PathEscape(projectID), url.PathEscape(observationID))
	var resp Observation
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateObservation(ctx context.Context, projectID string, input ObservationInput) (*Observation, error) {
	endpoint := fmt.Sprintf("/projects/%s/observations", url.PathEscape(projectID))
	var resp Observation
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SearchObservations(ctx context.Context, projectID, searchQuery, status, discoveredInEnvironmentKey, discoveredInEnvironmentVersion string) (*SearchObservationsResponse, error) {
	endpoint := fmt.Sprintf("/projects/%s/observations/search", url.PathEscape(projectID))
	query := url.Values{}
	if strings.TrimSpace(searchQuery) != "" {
		query.Set("query", strings.TrimSpace(searchQuery))
	}
	if strings.TrimSpace(status) != "" {
		query.Set("status", strings.TrimSpace(status))
	}
	if strings.TrimSpace(discoveredInEnvironmentKey) != "" {
		query.Set("discoveredInEnvironmentKey", strings.TrimSpace(discoveredInEnvironmentKey))
	}
	if strings.TrimSpace(discoveredInEnvironmentVersion) != "" {
		query.Set("discoveredInEnvironmentVersion", strings.TrimSpace(discoveredInEnvironmentVersion))
	}

	var resp SearchObservationsResponse
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) PromoteObservationToTicket(ctx context.Context, projectID, observationID string, input PromoteObservationToTicketInput) (*Issue, error) {
	endpoint := fmt.Sprintf("/projects/%s/observations/%s/promote-to-ticket", url.PathEscape(projectID), url.PathEscape(observationID))
	var resp Issue
	if err := c.doJSON(ctx, http.MethodPost, endpoint, input, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) PromoteObservationToWiki(ctx context.Context, projectID, observationID string, input PromoteObservationToWikiInput) error {
	endpoint := fmt.Sprintf("/projects/%s/observations/%s/promote-to-wiki", url.PathEscape(projectID), url.PathEscape(observationID))
	return c.doJSON(ctx, http.MethodPost, endpoint, input, nil, nil)
}

func (c *Client) ListExecutions(ctx context.Context, projectID string, params ListExecutionsParams) (*PagedResponse[ExecutionListItem], error) {
	endpoint := fmt.Sprintf("/projects/%s/executions", url.PathEscape(projectID))
	query := url.Values{}
	if strings.TrimSpace(params.Status) != "" {
		query.Set("status", strings.TrimSpace(params.Status))
	}
	if strings.TrimSpace(params.AgentID) != "" {
		query.Set("agentId", strings.TrimSpace(params.AgentID))
	}
	if strings.TrimSpace(params.EnvironmentKey) != "" {
		query.Set("environmentKey", strings.TrimSpace(params.EnvironmentKey))
	}
	query.Set("page", strconv.Itoa(params.Page))
	query.Set("pageSize", strconv.Itoa(params.PageSize))

	var resp PagedResponse[ExecutionListItem]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetExecution(ctx context.Context, projectID, executionID string) (*Execution, error) {
	endpoint := fmt.Sprintf("/projects/%s/executions/%s", url.PathEscape(projectID), url.PathEscape(executionID))
	var resp Execution
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RetryExecution(ctx context.Context, projectID, executionID string) (*Execution, error) {
	endpoint := fmt.Sprintf("/projects/%s/executions/%s/retry", url.PathEscape(projectID), url.PathEscape(executionID))
	var resp Execution
	if err := c.doJSON(ctx, http.MethodPost, endpoint, nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) StopExecution(ctx context.Context, projectID, executionID string) error {
	endpoint := fmt.Sprintf("/projects/%s/executions/%s/stop", url.PathEscape(projectID), url.PathEscape(executionID))
	return c.doJSON(ctx, http.MethodPost, endpoint, nil, nil, nil)
}

func (c *Client) ListExecutionsForIssue(ctx context.Context, projectID, issueID string, page, pageSize int) (*PagedResponse[Execution], error) {
	endpoint := fmt.Sprintf("/projects/%s/tickets/%s/executions", url.PathEscape(projectID), url.PathEscape(issueID))
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))

	var resp PagedResponse[Execution]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListExecutionArtifacts(ctx context.Context, projectID, executionID string, page, pageSize int) (*PagedResponse[ExecutionArtifact], error) {
	endpoint := fmt.Sprintf("/projects/%s/executions/%s/artifacts", url.PathEscape(projectID), url.PathEscape(executionID))
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))

	var resp PagedResponse[ExecutionArtifact]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListExecutionNotes(ctx context.Context, projectID, executionID string, page, pageSize int) (*PagedResponse[ExecutionNote], error) {
	endpoint := fmt.Sprintf("/projects/%s/executions/%s/notes", url.PathEscape(projectID), url.PathEscape(executionID))
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))

	var resp PagedResponse[ExecutionNote]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListExecutionTestCases(ctx context.Context, projectID, executionID string, page, pageSize int) (*PagedResponse[ExecutionTestCase], error) {
	endpoint := fmt.Sprintf("/projects/%s/executions/%s/testcases", url.PathEscape(projectID), url.PathEscape(executionID))
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))

	var resp PagedResponse[ExecutionTestCase]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListAgentSessionConversation(ctx context.Context, projectID, sessionID string, params ListConversationEventsParams) (*PagedResponse[ConversationEvent], error) {
	endpoint := fmt.Sprintf("/projects/%s/agentsessions/%s/conversation", url.PathEscape(projectID), url.PathEscape(sessionID))
	query := url.Values{}
	if params.Before != nil {
		query.Set("before", params.Before.UTC().Format(time.RFC3339Nano))
	}
	query.Set("page", strconv.Itoa(params.Page))
	query.Set("pageSize", strconv.Itoa(params.PageSize))

	var resp PagedResponse[ConversationEvent]
	if err := c.doJSON(ctx, http.MethodGet, endpointWithQuery(endpoint, query), nil, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) AskAdvisor(ctx context.Context, request AskAdvisorRequest) (*ChatResponse, error) {
	var resp ChatResponse
	if err := c.doJSON(ctx, http.MethodPost, "/chat/advisor", request, &resp, nil); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, body any, into any, headers map[string]string) error {
	if c.apiKey == "" && c.accessToken == "" {
		return errors.New("authentication is required")
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base url: %w", err)
	}
	pathPart := endpoint
	rawQuery := ""
	if idx := strings.Index(endpoint, "?"); idx >= 0 {
		pathPart = endpoint[:idx]
		rawQuery = endpoint[idx+1:]
	}
	u.Path = path.Join(u.Path, pathPart)
	u.RawQuery = rawQuery

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	} else if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp.StatusCode, respBody)
	}

	if into == nil || len(respBody) == 0 {
		return nil
	}

	if err := json.Unmarshal(respBody, into); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}

	return nil
}

func decodeAPIError(statusCode int, body []byte) error {
	apiErr := &APIError{StatusCode: statusCode}
	if len(body) == 0 {
		apiErr.Message = http.StatusText(statusCode)
		return apiErr
	}

	var parsed ErrorResponse
	if err := json.Unmarshal(body, &parsed); err == nil && strings.TrimSpace(parsed.Error) != "" {
		apiErr.Message = parsed.Error
		return apiErr
	}

	apiErr.Message = strings.TrimSpace(string(body))
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(statusCode)
	}

	return apiErr
}

func endpointWithQuery(endpoint string, query url.Values) string {
	if query == nil {
		return endpoint
	}
	encoded := query.Encode()
	if encoded == "" {
		return endpoint
	}
	return endpoint + "?" + encoded
}
