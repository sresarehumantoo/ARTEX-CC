package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func (s *Server) switchEngagementTool() tools.Tool {
	return tools.Tool{
		Name:     "switch_engagement",
		Category: tools.CategoryEngagement,
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

			reg := tools.NewRegistry()
			tools.RegisterAll(reg, s.store, eng.ID, eng.ExplorationID)
			if s.evidence != nil {
				tools.RegisterEvidenceTools(reg, s.evidence, s.store, eng.ID)
			}
			if s.traffic != nil {
				tools.RegisterTrafficTools(reg, s.traffic, s.store, eng.ID)
			}

			// Held across re-registration so concurrent switches cannot leave the
			// active IDs pointing at one engagement and the tools at another.
			s.engMu.Lock()
			s.taskID = eng.ID
			s.explorationID = eng.ExplorationID
			s.registry = reg
			for _, t := range reg.Tools() {
				s.addTool(t)
			}
			s.engMu.Unlock()

			return tools.Result{Content: fmt.Sprintf("switched to engagement %d (exploration %d): %s", eng.ID, eng.ExplorationID, eng.Goal)}, nil
		},
	}
}
