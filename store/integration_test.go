package store

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestTaskCRUD(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()

	task, err := s.CreateTask(ctx, "test task", "task description", "find bugs", nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.ID <= 0 {
		t.Fatalf("task.ID = %d, want > 0", task.ID)
	}
	if task.Name != "test task" {
		t.Errorf("Name = %q, want %q", task.Name, "test task")
	}

	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got == nil {
		t.Fatal("GetTask returned nil")
	}
	if got.Goal != "find bugs" {
		t.Errorf("Goal = %q, want %q", got.Goal, "find bugs")
	}

	tasks, err := s.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	found := false
	for _, tt := range tasks {
		if tt.ID == task.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListTasks did not include our task")
	}

	if err := s.SetTaskStatus(task.ID, "running"); err != nil {
		t.Fatalf("SetTaskStatus: %v", err)
	}
	updated, _ := s.GetTask(task.ID)
	if updated.Status != "running" {
		t.Errorf("Status = %q, want %q", updated.Status, "running")
	}
}

func TestGetTaskNonExistent(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	got, err := s.GetTask(999999)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent task, got %v", got)
	}
}

func TestListEngagements(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, _ = testEngagement(t, s, "list-engagements")

	engs, err := s.ListEngagements(ctx)
	if err != nil {
		t.Fatalf("ListEngagements: %v", err)
	}
	if len(engs) < 1 {
		t.Error("ListEngagements returned 0 results")
	}
}

func TestCompleteEngagement(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "complete-engagement")

	if err := s.CompleteEngagement(ctx, taskID, "done testing"); err != nil {
		t.Fatalf("CompleteEngagement: %v", err)
	}

	task, _ := s.GetTask(taskID)
	if task.Status != "done" {
		t.Errorf("Status = %q, want %q", task.Status, "done")
	}

	ov, err := s.GraphOverview(ctx, task.ExplorationID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if ov.Task["status"] != "achieved" {
		t.Errorf("overview status = %v, want %q", ov.Task["status"], "achieved")
	}
}

func TestCompleteEngagementNotFound(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	err := s.CompleteEngagement(ctx, 999999, "")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestUpdateEngagement(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "update-engagement")

	newGoal := "new goal"
	newDesc := "new description"
	if err := s.UpdateEngagement(ctx, taskID, &newGoal, &newDesc); err != nil {
		t.Fatalf("UpdateEngagement: %v", err)
	}

	task, _ := s.GetTask(taskID)
	if task.Goal != "new goal" {
		t.Errorf("Goal = %q, want %q", task.Goal, "new goal")
	}
	if task.Description != "new description" {
		t.Errorf("Description = %q, want %q", task.Description, "new description")
	}
}

func TestDeleteEngagement(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "delete-engagement")

	if err := s.DeleteEngagement(ctx, taskID); err != nil {
		t.Fatalf("DeleteEngagement: %v", err)
	}

	task, _ := s.GetTask(taskID)
	if task != nil {
		t.Error("task should be deleted")
	}
}

func TestDeleteEngagementNotFound(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	err := s.DeleteEngagement(ctx, 999999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestGetEngagement(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "get-engagement")

	detail, err := s.GetEngagement(ctx, taskID)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if detail == nil {
		t.Fatal("GetEngagement returned nil")
	}
	if detail.ID != taskID {
		t.Errorf("ID = %d, want %d", detail.ID, taskID)
	}
}

func TestGetEngagementNotFound(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	detail, err := s.GetEngagement(ctx, 999999)
	if err != nil {
		t.Fatalf("GetEngagement: %v", err)
	}
	if detail != nil {
		t.Error("expected nil for non-existent engagement")
	}
}

func TestNodeLifecycle(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "node-lifecycle")

	nodeID, err := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "test intent"}, 5, "open", "planner", nil)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	if err := s.UpdateNodeState(ctx, nodeID, "running"); err != nil {
		t.Fatalf("UpdateNodeState: %v", err)
	}

	if err := s.UpdateNodeFields(ctx, nodeID, "done", map[string]any{"result": "success"}); err != nil {
		t.Fatalf("UpdateNodeFields: %v", err)
	}

	detail, err := s.NodeDetail(ctx, nodeID)
	if err != nil {
		t.Fatalf("NodeDetail: %v", err)
	}
	if detail.State != "done" {
		t.Errorf("State = %q, want %q", detail.State, "done")
	}
	if detail.Payload["result"] != "success" {
		t.Errorf("Payload[result] = %v, want 'success'", detail.Payload["result"])
	}
}

func TestNodeDetailNotFound(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, err := s.NodeDetail(ctx, 999999)
	if err == nil {
		t.Error("expected error for non-existent node")
	}
}

func TestListNodes(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "list-nodes")

	_, _ = s.AddNode(ctx, expID, KindGoal, map[string]any{"text": "g1"}, 0, "open", "system", nil)
	_, _ = s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "i1"}, 5, "open", "planner", nil)

	all, err := s.ListNodes(ctx, expID, "", 0)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(all) < 3 {
		t.Errorf("ListNodes() = %d, want >= 3 (origin + goal + intent)", len(all))
	}

	goals, err := s.ListNodes(ctx, expID, KindGoal, 0)
	if err != nil {
		t.Fatalf("ListNodes(goal): %v", err)
	}
	if len(goals) < 1 {
		t.Error("expected at least 1 goal node")
	}
	for _, n := range goals {
		if n.Kind != KindGoal {
			t.Errorf("filtered node has kind %q, want %q", n.Kind, KindGoal)
		}
	}
}

func TestFrontier(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "frontier")

	_, _ = s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "i1"}, 10, "open", "planner", nil)
	_, _ = s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "i2"}, 5, "open", "planner", nil)
	_, _ = s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "done"}, 1, "done", "planner", nil)

	frontier, err := s.Frontier(ctx, expID, 10)
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(frontier) < 2 {
		t.Fatalf("Frontier returned %d, want >= 2", len(frontier))
	}
	for _, n := range frontier {
		if n.State != "open" {
			t.Errorf("frontier node has state %q, want open", n.State)
		}
	}
}

func TestCountIntents(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "count-intents")

	_, _ = s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "open"}, 5, "open", "planner", nil)
	_, _ = s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "done"}, 5, "done", "planner", nil)

	open, err := s.CountOpenIntents(ctx, expID)
	if err != nil {
		t.Fatalf("CountOpenIntents: %v", err)
	}
	if open < 1 {
		t.Errorf("CountOpenIntents = %d, want >= 1", open)
	}

	finished, err := s.CountFinishedIntents(ctx, expID)
	if err != nil {
		t.Fatalf("CountFinishedIntents: %v", err)
	}
	if finished < 1 {
		t.Errorf("CountFinishedIntents = %d, want >= 1", finished)
	}
}

func TestOriginFactID(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "origin-fact-id")

	id, err := s.OriginFactID(ctx, expID)
	if err != nil {
		t.Fatalf("OriginFactID: %v", err)
	}
	if id <= 0 {
		t.Errorf("OriginFactID = %d, want > 0", id)
	}
}

func TestEdgesRoundtrip(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "edges-roundtrip")

	g, _ := s.AddNode(ctx, expID, KindGoal, map[string]any{"text": "g"}, 0, "open", "system", nil)
	i, _ := s.AddNode(ctx, expID, KindIntent, map[string]any{"summary": "i"}, 5, "open", "planner", nil)
	f, _ := s.AddNode(ctx, expID, KindFact, map[string]any{"text": "f"}, 0, "confirmed", "worker", nil)

	err := s.AddEdges(ctx, expID, []Edge{
		{From: g, Rel: RelSpawns, To: i},
		{From: i, Rel: RelYields, To: f},
	})
	if err != nil {
		t.Fatalf("AddEdges: %v", err)
	}

	edges, err := s.Edges(ctx, expID, 100)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	if len(edges) < 2 {
		t.Errorf("Edges returned %d, want >= 2", len(edges))
	}
}

func TestAddEdgesEmpty(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "edges-empty")

	err := s.AddEdges(ctx, expID, nil)
	if err != nil {
		t.Fatalf("AddEdges(nil): %v", err)
	}
}

func TestConstraints(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "constraints")

	id, err := s.AddConstraint(ctx, expID, "deny", "no automated exploitation", "user")
	if err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	if id <= 0 {
		t.Fatalf("constraint ID = %d, want > 0", id)
	}

	constraints, err := s.ListConstraints(ctx, expID)
	if err != nil {
		t.Fatalf("ListConstraints: %v", err)
	}
	if len(constraints) < 1 {
		t.Fatal("ListConstraints returned 0 constraints")
	}
	if constraints[0].Text != "no automated exploitation" {
		t.Errorf("Text = %q", constraints[0].Text)
	}
}

func TestRootExploration(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "root-exploration")

	desc, goal, err := s.Root(ctx, expID)
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if desc == "" {
		t.Error("description should not be empty")
	}
	if goal == "" {
		t.Error("goal should not be empty")
	}
}

func TestFindingDismiss(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "finding-dismiss")

	findingID, _, err := s.AddFinding(ctx, tools.FindingInput{
		TaskID:        taskID,
		ExplorationID: expID,
		VulnClass:     "info_leak",
		Name:          "Verbose error messages",
		Severity:      SeverityLow,
		Summary:       "Stack traces in error responses",
	})
	if err != nil {
		t.Fatalf("AddFinding: %v", err)
	}

	if err := s.DismissFinding(ctx, findingID, "not exploitable"); err != nil {
		t.Fatalf("DismissFinding: %v", err)
	}

	f, err := s.GetFinding(findingID)
	if err != nil {
		t.Fatalf("GetFinding: %v", err)
	}
	if !f.Dismissed {
		t.Error("finding should be dismissed")
	}
	if f.DismissReason != "not exploitable" {
		t.Errorf("DismissReason = %q", f.DismissReason)
	}
	if f.Status != FindingStatusFalsePositive {
		t.Errorf("Status = %q, want %q", f.Status, FindingStatusFalsePositive)
	}

	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if ov.FindingsTotal != 0 || len(ov.FindingList) != 0 {
		t.Errorf("overview counts dismissed finding: total=%d list=%d", ov.FindingsTotal, len(ov.FindingList))
	}
}

func TestFindingDismissNotFound(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	err := s.DismissFinding(ctx, 999999, "reason")
	if err == nil {
		t.Error("expected error for non-existent finding")
	}
}

func TestFindingExists(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "finding-exists")

	findingID, _, _ := s.AddFinding(ctx, tools.FindingInput{
		TaskID:        taskID,
		ExplorationID: expID,
		VulnClass:     "xss",
		Name:          "Test XSS",
		Severity:      SeverityMedium,
		Summary:       "test",
	})

	exists, err := s.FindingExists(ctx, findingID)
	if err != nil {
		t.Fatalf("FindingExists: %v", err)
	}
	if !exists {
		t.Error("finding should exist")
	}

	exists, err = s.FindingExists(ctx, 999999)
	if err != nil {
		t.Fatalf("FindingExists(999999): %v", err)
	}
	if exists {
		t.Error("non-existent finding should not exist")
	}
}

func TestSetFindingStatus(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "set-finding-status")

	findingID, _, _ := s.AddFinding(ctx, tools.FindingInput{
		TaskID:        taskID,
		ExplorationID: expID,
		VulnClass:     "sqli",
		Name:          "SQL injection",
		Severity:      SeverityCritical,
		Summary:       "test",
	})

	if err := s.SetFindingStatus(findingID, FindingStatusConfirmed); err != nil {
		t.Fatalf("SetFindingStatus: %v", err)
	}
	f, _ := s.GetFinding(findingID)
	if f.Status != FindingStatusConfirmed {
		t.Errorf("Status = %q, want %q", f.Status, FindingStatusConfirmed)
	}
}

func TestListFindingsForReport(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "findings-report")

	_, _, _ = s.AddFinding(ctx, tools.FindingInput{
		TaskID:        taskID,
		ExplorationID: expID,
		VulnClass:     "xss",
		Name:          "Report XSS",
		Severity:      SeverityHigh,
		Summary:       "reflected xss",
		Evidence:      "proof",
	})

	findings, err := s.ListFindingsForReport(ctx, expID)
	if err != nil {
		t.Fatalf("ListFindingsForReport: %v", err)
	}
	if len(findings) < 1 {
		t.Fatal("expected at least 1 finding for report")
	}
	if findings[0].VulnClass != "xss" {
		t.Errorf("VulnClass = %q, want %q", findings[0].VulnClass, "xss")
	}
}

func TestTaskScope(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "task-scope")

	if err := s.AddTaskScope(ctx, taskID, "root_domain", "scope-test.example.com", "", "agent"); err != nil {
		t.Fatalf("AddTaskScope root_domain: %v", err)
	}
	if err := s.AddTaskScope(ctx, taskID, "ip", "10.10.10.1", "recon found it", "agent"); err != nil {
		t.Fatalf("AddTaskScope ip: %v", err)
	}

	ids, err := s.ListTaskScope(ctx, taskID)
	if err != nil {
		t.Fatalf("ListTaskScope: %v", err)
	}
	if len(ids) < 2 {
		t.Fatalf("ListTaskScope returned %d, want >= 2", len(ids))
	}

	inScope, reason, _, err := s.IsInScope(ctx, taskID, "sub.scope-test.example.com")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if !inScope {
		t.Errorf("sub.scope-test.example.com should be in scope, reason: %s", reason)
	}

	inScope, _, _, err = s.IsInScope(ctx, taskID, "10.10.10.1")
	if err != nil {
		t.Fatalf("IsInScope IP: %v", err)
	}
	if !inScope {
		t.Error("10.10.10.1 should be in scope")
	}

	inScope, _, _, err = s.IsInScope(ctx, taskID, "out-of-scope.example.org")
	if err != nil {
		t.Fatalf("IsInScope out: %v", err)
	}
	if inScope {
		t.Error("out-of-scope.example.org should not be in scope")
	}
}

func TestTaskScopeValidation(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "task-scope-validate")

	if err := s.AddTaskScope(ctx, 0, "root_domain", "example.com", "", "agent"); err == nil {
		t.Error("expected error for task_id=0")
	}
	if err := s.AddTaskScope(ctx, taskID, "root_domain", "", "", "agent"); err == nil {
		t.Error("expected error for empty value")
	}
	if err := s.AddTaskScope(ctx, taskID, "unknown_kind", "value", "", "agent"); err == nil {
		t.Error("expected error for unknown kind")
	}
	if err := s.AddTaskScope(ctx, taskID, "ip", "not-an-ip", "", "agent"); err == nil {
		t.Error("expected error for invalid IP")
	}
	if err := s.AddTaskScope(ctx, taskID, "cidr", "not-a-cidr", "", "agent"); err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestDeleteTaskScope(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "delete-scope")

	_ = s.AddTaskScope(ctx, taskID, "root_domain", "delete-me.example.com", "", "agent")
	ids, _ := s.ListTaskScope(ctx, taskID)
	if len(ids) < 1 {
		t.Fatal("expected at least 1 scope entry")
	}

	if err := s.DeleteTaskScope(ids[0], taskID); err != nil {
		t.Fatalf("DeleteTaskScope: %v", err)
	}

	err := s.DeleteTaskScope(999999, taskID)
	if err == nil {
		t.Error("expected error for non-existent scope entry")
	}
}

func TestIsInScopeNoRules(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "scope-no-rules")

	inScope, reason, _, err := s.IsInScope(ctx, taskID, "anything.com")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if !inScope {
		t.Errorf("should be in scope with no rules, reason: %s", reason)
	}
}

func TestIsInScopeEmptyTarget(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()

	inScope, _, _, err := s.IsInScope(ctx, 1, "")
	if err != nil {
		t.Fatalf("IsInScope: %v", err)
	}
	if inScope {
		t.Error("empty target should not be in scope")
	}
}

func TestCredentialsEncrypted(t *testing.T) {
	t.Parallel()
	// A private Store so the key never leaks onto the shared one used by parallel tests.
	s := &Store{DB: testDB(t).DB}
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "creds-encrypted")

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	s.SetCredentialKey(key)

	_, err := s.SaveCredential(ctx, taskID, tools.CredentialInput{
		Domain:   fmt.Sprintf("enc-test-%d.com", os.Getpid()),
		Username: "admin",
		Password: "s3cr3t",
		Source:   "agent",
	})
	if err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}

	creds, err := s.ListCredentials(ctx, taskID)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	found := false
	for _, c := range creds {
		if c.Username == "admin" {
			found = true
			if c.Password != "s3cr3t" {
				t.Errorf("decrypted Password = %q, want %q", c.Password, "s3cr3t")
			}
		}
	}
	if !found {
		t.Error("credential not found in list")
	}
}

func TestCredentialValidation(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "cred-validate")

	_, err := s.SaveCredential(ctx, taskID, tools.CredentialInput{
		Domain:   "",
		Username: "user",
		Password: "pass",
	})
	if err == nil {
		t.Error("expected error for empty domain")
	}

	_, err = s.SaveCredential(ctx, taskID, tools.CredentialInput{
		Domain:   "example.com",
		Username: "",
		Password: "pass",
	})
	if err == nil {
		t.Error("expected error for empty username")
	}

	_, err = s.SaveCredential(ctx, taskID, tools.CredentialInput{
		Domain:   "example.com",
		Username: "user",
		Password: "",
		Source:   "agent",
	})
	if err == nil {
		t.Error("expected error for agent credential with empty password")
	}
}

func TestCoverageRecordValidation(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "coverage-validate")

	err := s.RecordCoverage(ctx, 0, tools.CoverageEntry{
		Surface: "s", RiskArea: "r", Outcome: OutcomeReported,
	})
	if err == nil {
		t.Error("expected error for task_id=0")
	}

	err = s.RecordCoverage(ctx, taskID, tools.CoverageEntry{
		Surface: "", RiskArea: "r", Outcome: OutcomeReported,
	})
	if err == nil {
		t.Error("expected error for empty surface")
	}

	err = s.RecordCoverage(ctx, taskID, tools.CoverageEntry{
		Surface: "s", RiskArea: "r", Outcome: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid outcome")
	}
}

func TestCoverageStats(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "coverage-stats")

	_ = s.RecordCoverage(ctx, taskID, tools.CoverageEntry{
		Surface: "login", RiskArea: "auth", Outcome: OutcomeReported,
		Evidence: "brute force", Worker: "test",
	})
	_ = s.RecordCoverage(ctx, taskID, tools.CoverageEntry{
		Surface: "api", RiskArea: "injection", Outcome: OutcomeNeedsFollowUp,
		Worker: "test",
	})

	stats, err := s.CoverageStats(taskID)
	if err != nil {
		t.Fatalf("CoverageStats: %v", err)
	}
	if len(stats.Entries) < 2 {
		t.Errorf("Entries = %d, want >= 2", len(stats.Entries))
	}
	gapFound := false
	for _, g := range stats.Gaps {
		if g.Kind == GapNeedsFollowUp && g.RiskArea == "injection" {
			gapFound = true
		}
	}
	if !gapFound {
		t.Error("expected needs_follow_up gap for injection")
	}
}

func TestDeleteAssetsByHost(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "delete-assets-host")

	host := fmt.Sprintf("delete-host-%d.com", os.Getpid())
	_, _ = s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: host})
	_, _ = s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "subdomain", Domain: "sub." + host})

	n, err := s.DeleteAssetsByHost(host)
	if err != nil {
		t.Fatalf("DeleteAssetsByHost: %v", err)
	}
	if n < 1 {
		t.Errorf("deleted %d, want >= 1", n)
	}
}

func TestDeleteAssetsByHostEmpty(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	_, err := s.DeleteAssetsByHost("")
	if err == nil {
		t.Error("expected error for empty host")
	}
}

func TestLogAndListActivity(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "activity-log")

	id, err := s.LogActivity(ctx, expID, tools.ActivityEntry{
		Kind:    "test",
		Summary: "did something",
		Worker:  "test-worker",
	})
	if err != nil {
		t.Fatalf("LogActivity: %v", err)
	}
	if id <= 0 {
		t.Fatalf("activity ID = %d, want > 0", id)
	}

	activities, err := s.ListActivity(ctx, expID, tools.ActivityFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(activities) < 1 {
		t.Fatal("ListActivity returned 0 activities")
	}
}

func TestListActivityFiltered(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, expID := testEngagement(t, s, "activity-filter")

	nodeID, _ := s.AddNode(ctx, expID, KindFact, map[string]any{"text": "test"}, 0, "confirmed", "worker", nil)
	_, _ = s.LogActivity(ctx, expID, tools.ActivityEntry{
		NodeID:  nodeID,
		Kind:    "tool_call",
		Summary: "ran nmap",
		Worker:  "worker",
	})
	_, _ = s.LogActivity(ctx, expID, tools.ActivityEntry{
		Kind:    "other",
		Summary: "something else",
		Worker:  "worker",
	})

	byKind, err := s.ListActivity(ctx, expID, tools.ActivityFilter{Kind: "tool_call", Limit: 10})
	if err != nil {
		t.Fatalf("ListActivity by kind: %v", err)
	}
	for _, a := range byKind {
		if a.Kind != "tool_call" {
			t.Errorf("filtered activity has kind %q, want %q", a.Kind, "tool_call")
		}
	}

	byNode, err := s.ListActivity(ctx, expID, tools.ActivityFilter{NodeID: nodeID, Limit: 10})
	if err != nil {
		t.Fatalf("ListActivity by node: %v", err)
	}
	for _, a := range byNode {
		if a.NodeID != nodeID {
			t.Errorf("filtered activity has NodeID %d, want %d", a.NodeID, nodeID)
		}
	}
}

func TestEngagementActivity(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "engagement-activity")

	_, _, _ = s.AddFinding(ctx, tools.FindingInput{
		TaskID: taskID, ExplorationID: expID,
		VulnClass: "xss", Name: "xss1", Severity: SeverityHigh, Summary: "test xss",
	})

	timeline, err := s.EngagementActivity(ctx, expID, 20)
	if err != nil {
		t.Fatalf("EngagementActivity: %v", err)
	}
	if len(timeline) < 1 {
		t.Error("expected at least 1 timeline entry")
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "export-import")

	_, _ = s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: fmt.Sprintf("export-test-%d.com", os.Getpid())})
	_, _, _ = s.AddFinding(ctx, tools.FindingInput{
		TaskID: taskID, ExplorationID: expID,
		VulnClass: "xss", Name: "Export XSS", Severity: SeverityHigh, Summary: "test",
	})
	_ = s.RecordCoverage(ctx, taskID, tools.CoverageEntry{
		Surface: "test", RiskArea: "xss", Outcome: OutcomeReported, Evidence: "proof", Worker: "test",
	})

	export, err := s.ExportEngagementData(ctx, taskID)
	if err != nil {
		t.Fatalf("ExportEngagementData: %v", err)
	}
	if export.Goal == "" {
		t.Error("exported goal is empty")
	}
	if len(export.Nodes) < 1 {
		t.Error("exported nodes are empty")
	}

	newTaskID, newExpID, err := s.ImportEngagementData(ctx, export)
	if err != nil {
		t.Fatalf("ImportEngagementData: %v", err)
	}
	if newTaskID <= 0 || newExpID <= 0 {
		t.Errorf("imported IDs: task=%d exp=%d", newTaskID, newExpID)
	}

	reimport, err := s.ExportEngagementData(ctx, newTaskID)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if reimport.Goal != export.Goal {
		t.Errorf("re-exported goal = %q, want %q", reimport.Goal, export.Goal)
	}
}

func TestExportNonExistent(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, err := s.ExportEngagementData(ctx, 999999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestImportInvalidVersion(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	_, _, err := s.ImportEngagementData(ctx, &tools.EngagementExport{Version: 0})
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestMergeFindings(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "merge-findings")

	id1, _, _ := s.AddFinding(ctx, tools.FindingInput{
		TaskID: taskID, ExplorationID: expID,
		VulnClass: "xss", Name: "XSS 1", Severity: SeverityHigh,
		Summary: "first xss", Evidence: "evidence 1",
	})
	id2, _, _ := s.AddFinding(ctx, tools.FindingInput{
		TaskID: taskID, ExplorationID: expID,
		VulnClass: "xss", Name: "XSS 2", Severity: SeverityHigh,
		Summary: "second xss", Evidence: "evidence 2",
	})

	merged, err := s.MergeFindings(ctx, id1, []int64{id2})
	if err != nil {
		t.Fatalf("MergeFindings: %v", err)
	}
	if merged.ID != id1 {
		t.Errorf("merged ID = %d, want %d", merged.ID, id1)
	}

	secondary, _ := s.GetFinding(id2)
	if secondary.MergedInto == nil || *secondary.MergedInto != id1 {
		t.Errorf("secondary MergedInto = %v, want %d", secondary.MergedInto, id1)
	}
	if secondary.Status != FindingStatusDuplicate {
		t.Errorf("secondary Status = %q, want %q", secondary.Status, FindingStatusDuplicate)
	}

	ov, err := s.GraphOverview(ctx, expID)
	if err != nil {
		t.Fatalf("GraphOverview: %v", err)
	}
	if ov.FindingsTotal != 1 || len(ov.FindingList) != 1 {
		t.Errorf("overview after merge: total=%d list=%d, want 1 and 1", ov.FindingsTotal, len(ov.FindingList))
	}
}

func TestFindingTrafficBindings(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "finding-traffic")

	findingID, _, _ := s.AddFinding(ctx, tools.FindingInput{
		TaskID: taskID, ExplorationID: expID,
		VulnClass: "xss", Name: "Traffic XSS", Severity: SeverityHigh, Summary: "test",
	})

	if err := s.BindFindingTraffic(ctx, findingID, "snap-123", "request", "initial req", 0); err != nil {
		t.Fatalf("BindFindingTraffic: %v", err)
	}

	bindings, err := s.ListFindingTraffic(ctx, findingID)
	if err != nil {
		t.Fatalf("ListFindingTraffic: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("got %d bindings, want 1", len(bindings))
	}
	if bindings[0].SnapshotID != "snap-123" {
		t.Errorf("SnapshotID = %q, want %q", bindings[0].SnapshotID, "snap-123")
	}

	count, err := s.FindingTrafficCount(ctx, findingID)
	if err != nil {
		t.Fatalf("FindingTrafficCount: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestListCoverageGaps(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, expID := testEngagement(t, s, "coverage-gaps")

	_, _ = s.InsertAsset(ctx, taskID, tools.AssetInput{
		Type:   "service",
		URL:    fmt.Sprintf("https://gaps-test-%d.com/api", os.Getpid()),
		Domain: fmt.Sprintf("gaps-test-%d.com", os.Getpid()),
	})

	gaps, err := s.ListCoverageGaps(ctx, taskID, expID)
	if err != nil {
		t.Fatalf("ListCoverageGaps: %v", err)
	}
	if gaps == nil {
		t.Fatal("ListCoverageGaps returned nil")
	}
}

func TestAssetTypeVariants(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "asset-type-variants")

	p := 22
	assets := []tools.AssetInput{
		{Type: "root_domain", Domain: fmt.Sprintf("variant-%d.com", os.Getpid())},
		{Type: "ip", IP: "198.51.100.42"},
		{Type: "subdomain", Domain: fmt.Sprintf("sub.variant-%d.com", os.Getpid()), IP: "198.51.100.42"},
		{Type: "service", URL: fmt.Sprintf("https://variant-%d.com/app", os.Getpid()), Domain: fmt.Sprintf("variant-%d.com", os.Getpid())},
		{Type: "service", ServiceName: "ssh", IP: "198.51.100.42", Port: p},
		{Type: "endpoint", URL: fmt.Sprintf("https://variant-%d.com/api/v1", os.Getpid()), Method: "POST"},
		{Type: "app", BundleID: fmt.Sprintf("com.test.variant%d", os.Getpid())},
		{Type: "app", AppName: fmt.Sprintf("Test Variant App %d", os.Getpid())},
	}
	for i, a := range assets {
		id, err := s.InsertAsset(ctx, taskID, a)
		if err != nil {
			t.Fatalf("InsertAsset[%d] (%s): %v", i, a.Type, err)
		}
		if id <= 0 {
			t.Errorf("InsertAsset[%d] id = %d", i, id)
		}
	}
}

func TestInsertHTTPServiceTwiceWithoutTechnologies(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "http-service-twice")

	svc := tools.AssetInput{Type: "service", URL: fmt.Sprintf("https://twice-%d.example/", os.Getpid())}
	first, err := s.InsertAsset(ctx, taskID, svc)
	if err != nil {
		t.Fatalf("first InsertAsset: %v", err)
	}
	second, err := s.InsertAsset(ctx, taskID, svc)
	if err != nil {
		t.Fatalf("second InsertAsset: %v", err)
	}
	if second != first {
		t.Errorf("second insert id = %d, want %d", second, first)
	}
}

func TestInsertAssetUnknownType(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "asset-unknown-type")

	_, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "unknown_type"})
	if err == nil {
		t.Error("expected error for unknown asset type")
	}
}

func TestInsertAssetValidation(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "asset-validation")

	_, err := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: ""})
	if err == nil {
		t.Error("expected error for root_domain with empty domain")
	}

	_, err = s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "ip", IP: ""})
	if err == nil {
		t.Error("expected error for ip with empty IP")
	}

	_, err = s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "endpoint", URL: "", Method: ""})
	if err == nil {
		t.Error("expected error for endpoint with empty url/method")
	}

	_, err = s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "app"})
	if err == nil {
		t.Error("expected error for app with no bundle_id or app_name")
	}

	_, err = s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "service", ServiceName: ""})
	if err == nil {
		t.Error("expected error for other service with empty service_name")
	}
}

func TestListAssetsFilter(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "list-assets-filter")

	domain := fmt.Sprintf("filter-test-%d.com", os.Getpid())
	id, _ := s.InsertAsset(ctx, taskID, tools.AssetInput{Type: "root_domain", Domain: domain})

	byID, total, err := s.ListAssets(ctx, tools.AssetFilter{ID: id})
	if err != nil {
		t.Fatalf("ListAssets by ID: %v", err)
	}
	if total != 1 || len(byID) != 1 {
		t.Errorf("by ID: total=%d len=%d", total, len(byID))
	}

	byIDs, _, err := s.ListAssets(ctx, tools.AssetFilter{IDs: []int64{id}})
	if err != nil {
		t.Fatalf("ListAssets by IDs: %v", err)
	}
	if len(byIDs) != 1 {
		t.Errorf("by IDs: len=%d", len(byIDs))
	}

	byType, _, err := s.ListAssets(ctx, tools.AssetFilter{TaskID: taskID, Type: "root_domain", Limit: 50})
	if err != nil {
		t.Fatalf("ListAssets by type: %v", err)
	}
	if len(byType) < 1 {
		t.Error("ListAssets by type returned 0")
	}
}

func TestListAssetsNonExistentID(t *testing.T) {
	t.Parallel()
	s := testDB(t)
	ctx := context.Background()
	assets, total, err := s.ListAssets(ctx, tools.AssetFilter{ID: 999999})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if total != 0 || len(assets) != 0 {
		t.Errorf("expected 0 results, got total=%d len=%d", total, len(assets))
	}
}

func TestInterceptRules(t *testing.T) {
	t.Parallel()
	s := testDB(t)

	rule := &InterceptRule{
		Name:        fmt.Sprintf("test-rule-%d", os.Getpid()),
		Enabled:     true,
		Priority:    10,
		MatchTarget: "tool_name",
		MatchType:   "string",
		Pattern:     "dangerous",
		Action:      "ask",
		Message:     "are you sure?",
	}
	if err := s.AddInterceptRule(rule); err != nil {
		t.Fatalf("AddInterceptRule: %v", err)
	}
	if rule.ID <= 0 {
		t.Errorf("rule.ID = %d, want > 0", rule.ID)
	}

	rules, err := s.ListInterceptRules()
	if err != nil {
		t.Fatalf("ListInterceptRules: %v", err)
	}
	found := false
	for _, r := range rules {
		if r.ID == rule.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("rule not found in ListInterceptRules")
	}

	if err := s.DeleteInterceptRule(rule.ID); err != nil {
		t.Fatalf("DeleteInterceptRule: %v", err)
	}
}
