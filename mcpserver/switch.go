package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SREsAreHumanToo/cc-artex/tools"
)

func (s *Server) registerSwitchEngagement() {
	t := tools.Tool{
		Name: "switch_engagement",
		Description: "Switch to a different engagement without restarting the server. " +
			"All tools will be re-registered to operate on the new engagement.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "integer", "description": "task id of the engagement to switch to"},
			},
			"required": []string{"task_id"},
		},
		Run: func(ctx context.Context, in json.RawMessage) (tools.Result, error) {
			var a struct {
				TaskID int64 `json:"task_id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return tools.Result{Content: "invalid input: " + err.Error(), IsError: true}, nil
			}
			if a.TaskID <= 0 {
				return tools.Result{Content: "task_id is required", IsError: true}, nil
			}

			eng, err := s.store.GetEngagement(ctx, a.TaskID)
			if err != nil {
				return tools.Result{Content: err.Error(), IsError: true}, nil
			}
			if eng == nil {
				return tools.Result{Content: fmt.Sprintf("engagement %d not found", a.TaskID), IsError: true}, nil
			}

			s.taskID = eng.ID
			s.explorationID = eng.ExplorationID

			reg := tools.NewRegistry()
			tools.RegisterAll(reg, s.store, s.taskID, s.explorationID)
			if s.evidence != nil {
				tools.RegisterEvidenceTools(reg, s.evidence, s.store, s.taskID)
			}
			if s.traffic != nil {
				tools.RegisterTrafficTools(reg, s.traffic, s.store, s.taskID)
			}
			s.registry = reg
			for _, t := range reg.Tools() {
				s.mcp.AddTool(convertTool(t), makeHandler(t))
			}

			return tools.Result{Content: fmt.Sprintf("switched to engagement %d (exploration %d): %s", s.taskID, s.explorationID, eng.Goal)}, nil
		},
	}
	s.mcp.AddTool(convertTool(t), makeHandler(t))
}
