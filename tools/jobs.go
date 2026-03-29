package tools

import (
	"context"
	"fmt"

	"video-pipeline-mcp/jobs"
	"video-pipeline-mcp/util"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerListJobs(s *server.MCPServer, queue *jobs.Queue) {
	tool := mcp.NewTool("list_jobs",
		mcp.WithDescription("List all jobs with their current status and progress."),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobList := queue.List()

		type jobInfo struct {
			ID       string `json:"id"`
			Tool     string `json:"tool"`
			Status   string `json:"status"`
			Progress string `json:"progress"`
			Output   string `json:"output,omitempty"`
			Error    string `json:"error,omitempty"`
		}

		var result []jobInfo
		for _, j := range jobList {
			result = append(result, jobInfo{
				ID:       j.ID,
				Tool:     j.Tool,
				Status:   string(j.Status),
				Progress: j.Progress,
				Output:   j.Output,
				Error:    j.Error,
			})
		}

		if len(result) == 0 {
			return util.TextResult("No jobs found."), nil
		}

		return util.SuccessResult(result)
	})
}

func registerGetJobStatus(s *server.MCPServer, queue *jobs.Queue) {
	tool := mcp.NewTool("get_job_status",
		mcp.WithDescription("Get the current status and details of a specific job."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Job ID (e.g. 'job_abc12345')"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := util.GetStringArg(req.Params.Arguments, "id")
		if id == "" {
			return util.ErrorResult(util.ErrInvalidInput, "id is required", "get_job_status"), nil
		}

		job, ok := queue.Get(id)
		if !ok {
			return util.ErrorResult(util.ErrJobNotFound,
				fmt.Sprintf("Job '%s' not found. Use list_jobs to see all jobs.", id),
				"get_job_status"), nil
		}

		result := map[string]interface{}{
			"id":       job.ID,
			"tool":     job.Tool,
			"status":   string(job.Status),
			"progress": job.Progress,
			"output":   job.Output,
			"error":    job.Error,
		}

		return util.SuccessResult(result)
	})
}

func registerCancelJob(s *server.MCPServer, queue *jobs.Queue) {
	tool := mcp.NewTool("cancel_job",
		mcp.WithDescription("Cancel a running or queued job."),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Job ID to cancel"),
		),
	)

	addTool(s, tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := util.GetStringArg(req.Params.Arguments, "id")
		if id == "" {
			return util.ErrorResult(util.ErrInvalidInput, "id is required", "cancel_job"), nil
		}

		ok := queue.Cancel(id)
		if !ok {
			return util.ErrorResult(util.ErrJobNotFound,
				fmt.Sprintf("Job '%s' not found or not cancellable (already done/failed).", id),
				"cancel_job"), nil
		}

		result := map[string]interface{}{
			"status":  "ok",
			"id":      id,
			"message": fmt.Sprintf("Job '%s' has been cancelled.", id),
		}
		return util.SuccessResult(result)
	})
}
