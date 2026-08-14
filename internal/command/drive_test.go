package command

import (
	"strings"
	"testing"

	"xynapse/internal/config"
	"xynapse/internal/git"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func boolPtr(b bool) *bool { return &b }

func driveTestConfig() *config.Config {
	return &config.Config{
		Defaults: config.Defaults{Project: "PROJ", BoardID: "99"},
		Git: config.GitConfig{
			Dir:             "",
			BranchTemplate:  "feature-v5/{Key}",
			BranchTemplates: map[string]string{"Bug": "fix-v5/{Key}"},
		},
		Workflow: config.WorkflowConfig{
			TestCommand:    "go test ./...",
			LintCommand:    "go vet ./...",
			TestStatus:     "In Review",
			BaseBranch:     "main",
			TargetBranch:   "develop",
			CommentTemplate: "PR: {url}",
		},
		Storage: config.StorageConfig{Base: "storage"},
	}
}

func TestSelectedStepsAll(t *testing.T) {
	got, err := selectedSteps(DriveOptions{})
	if err != nil {
		t.Fatalf("selectedSteps: %v", err)
	}
	want := strings.Join(driveSteps, ",")
	if strings.Join(got, ",") != want {
		t.Errorf("selectedSteps = %v, want %v", got, driveSteps)
	}
}

func TestSelectedStepsSingle(t *testing.T) {
	got, err := selectedSteps(DriveOptions{Step: "test"})
	if err != nil {
		t.Fatalf("selectedSteps: %v", err)
	}
	if len(got) != 1 || got[0] != "test" {
		t.Errorf("selectedSteps(single) = %v, want [test]", got)
	}
}

func TestSelectedStepsRange(t *testing.T) {
	got, err := selectedSteps(DriveOptions{From: "plan", To: "test"})
	if err != nil {
		t.Fatalf("selectedSteps: %v", err)
	}
	if strings.Join(got, ",") != "plan,implement,test" {
		t.Errorf("selectedSteps(range) = %v", got)
	}
}

func TestSelectedStepsErrors(t *testing.T) {
	for _, tc := range []DriveOptions{
		{Step: "bogus"},
		{From: "bogus"},
		{To: "bogus"},
		{From: "test", To: "plan"},
	} {
		if _, err := selectedSteps(tc); err == nil {
			t.Errorf("selectedSteps(%+v) expected error", tc)
		}
	}
}

func TestDrivePRTarget(t *testing.T) {
	cfg := driveTestConfig()
	if got := drivePRTarget(cfg, "", ""); got != "develop" {
		t.Errorf("drivePRTarget = %q, want develop (target_branch)", got)
	}
	if got := drivePRTarget(cfg, "main", ""); got != "main" {
		t.Errorf("drivePRTarget(flag) = %q, want main", got)
	}
	// Fallback to base_branch when target_branch is empty.
	cfg.Workflow.TargetBranch = ""
	if got := drivePRTarget(cfg, "", ""); got != "main" {
		t.Errorf("drivePRTarget = %q, want main (base_branch fallback)", got)
	}
}

func TestTestStatus(t *testing.T) {
	cfg := driveTestConfig()
	if got := testStatus(cfg, ""); got != "In Review" {
		t.Errorf("testStatus = %q, want In Review", got)
	}
	if got := testStatus(cfg, "QA"); got != "QA" {
		t.Errorf("testStatus(flag) = %q, want QA", got)
	}
}

func TestDerivedBranch(t *testing.T) {
	cfg := driveTestConfig()
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ", Type: "Story"}
	branch, err := derivedBranch(cfg, ticket)
	if err != nil {
		t.Fatalf("derivedBranch: %v", err)
	}
	if branch != "feature-v5/PROJ-1" {
		t.Errorf("derivedBranch = %q, want feature-v5/PROJ-1", branch)
	}

	// Bug type uses the per-type template.
	cfg.Git.BranchTemplates["Bug"] = "fix-v5/{Key}"
	branch, err = derivedBranch(cfg, &models.Ticket{Key: "PROJ-2", Project: "PROJ", Type: "Bug"})
	if err != nil {
		t.Fatalf("derivedBranch: %v", err)
	}
	if branch != "fix-v5/PROJ-2" {
		t.Errorf("derivedBranch(bug) = %q, want fix-v5/PROJ-2", branch)
	}
}

func TestRenderCommentTemplate(t *testing.T) {
	cfg := driveTestConfig()
	cfg.Workflow.CommentTemplate = "PR: {url} closes {key} [{branch}]"
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ", Summary: "Add x"}
	pr := &git.PRDetails{URL: "https://github.com/x/y/pull/1"}
	got := renderCommentTemplate(cfg, ticket, pr)
	for _, want := range []string{"PR: https://github.com/x/y/pull/1", "closes PROJ-1", "[feature-v5/PROJ-1]"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderCommentTemplate missing %q: %q", want, got)
		}
	}
}

func TestRenderCommentTemplateDefault(t *testing.T) {
	cfg := driveTestConfig()
	cfg.Workflow.CommentTemplate = ""
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	got := renderCommentTemplate(cfg, ticket, &git.PRDetails{URL: "https://x/pull/1"})
	if !strings.Contains(got, "Closes PROJ-1") {
		t.Errorf("default template should include Closes key: %q", got)
	}
}

func TestDriveTestStepSkipsEmptyCommand(t *testing.T) {
	cfg := driveTestConfig()
	cfg.Workflow.TestCommand = "  "
	s := storage.NewStorage(t.TempDir())
	ctx := &driveCtx{cfg: cfg, s: s, g: git.New(t.TempDir(), ""), opts: DriveOptions{}}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	run, err := driveTestStep(ctx, ticket, stepTest)
	if err != nil {
		t.Fatalf("driveTestStep: %v", err)
	}
	if run {
		t.Error("empty command should skip the step")
	}
}

func TestDriveTestStepPass(t *testing.T) {
	cfg := driveTestConfig()
	cfg.Workflow.TestCommand = "echo ok"
	s := storage.NewStorage(t.TempDir())
	ctx := &driveCtx{cfg: cfg, s: s, g: git.New(t.TempDir(), ""), opts: DriveOptions{}}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	run, err := driveTestStep(ctx, ticket, stepTest)
	if err != nil {
		t.Fatalf("driveTestStep: %v", err)
	}
	if !run {
		t.Error("passing command should complete the step")
	}
	if _, failed := s.DriveStepFailed(ticket.Project, ticket.Key, stepTest); failed {
		t.Error("no failure should be recorded for a passing command")
	}
}

func TestDriveTestStepFail(t *testing.T) {
	cfg := driveTestConfig()
	cfg.Workflow.TestCommand = "exit 1"
	s := storage.NewStorage(t.TempDir())
	ctx := &driveCtx{cfg: cfg, s: s, g: git.New(t.TempDir(), ""), opts: DriveOptions{}}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	run, err := driveTestStep(ctx, ticket, stepTest)
	if err == nil {
		t.Fatal("expected error for failing command")
	}
	if run {
		t.Error("failing command must not complete the step")
	}
	if _, failed := s.DriveStepFailed(ticket.Project, ticket.Key, stepTest); !failed {
		t.Error("failure should be recorded")
	}
}

func TestDriveTestStepFailForceBypasses(t *testing.T) {
	cfg := driveTestConfig()
	cfg.Workflow.TestCommand = "exit 1"
	s := storage.NewStorage(t.TempDir())
	ctx := &driveCtx{cfg: cfg, s: s, g: git.New(t.TempDir(), ""), opts: DriveOptions{Force: true}}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	run, err := driveTestStep(ctx, ticket, stepTest)
	if err != nil {
		t.Fatalf("driveTestStep with force: %v", err)
	}
	if run {
		t.Error("force-bypassed failure should not mark the step done")
	}
	if _, failed := s.DriveStepFailed(ticket.Project, ticket.Key, stepTest); !failed {
		t.Error("failure should still be recorded under --force")
	}
}

func driveACSuccess() string {
	return `Implementation complete.

## AC Results

- [x] AC one
- [x] AC two
`
}

func driveACPartial() string {
	return `Implementation complete.

## AC Results

- [x] AC one
- [ ] AC two
`
}

func TestDriveCheckACAllPass(t *testing.T) {
	s := storage.NewStorage(t.TempDir())
	if err := s.WriteReport("PROJ", "PROJ-1", driveACSuccess()); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	ctx := &driveCtx{s: s}
	_, failed := driveCheckAC(ctx, ticket)
	if failed {
		t.Error("all-pass ACs should not fail the gate")
	}
}

func TestDriveCheckACPartialFails(t *testing.T) {
	s := storage.NewStorage(t.TempDir())
	if err := s.WriteReport("PROJ", "PROJ-1", driveACPartial()); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	ctx := &driveCtx{s: s}
	_, failed := driveCheckAC(ctx, ticket)
	if !failed {
		t.Error("partial ACs should fail the gate")
	}
}

func TestDriveCheckACMissingReportFails(t *testing.T) {
	s := storage.NewStorage(t.TempDir())
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	ctx := &driveCtx{s: s}
	_, failed := driveCheckAC(ctx, ticket)
	if !failed {
		t.Error("missing report should fail the gate")
	}
}

func TestDriveCheckACNoACSectionFails(t *testing.T) {
	s := storage.NewStorage(t.TempDir())
	if err := s.WriteReport("PROJ", "PROJ-1", "# done\n"); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	ctx := &driveCtx{s: s}
	_, failed := driveCheckAC(ctx, ticket)
	if !failed {
		t.Error("report without AC Results section should fail the gate")
	}
}

func TestDriveCheckACClearRecordedFailure(t *testing.T) {
	s := storage.NewStorage(t.TempDir())
	if err := s.SetDriveFailure("PROJ", "PROJ-1", stepAC, "old failure"); err != nil {
		t.Fatalf("SetDriveFailure: %v", err)
	}
	if err := s.WriteReport("PROJ", "PROJ-1", driveACSuccess()); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	ticket := &models.Ticket{Key: "PROJ-1", Project: "PROJ"}
	ctx := &driveCtx{s: s}
	_, failed := driveCheckAC(ctx, ticket)
	if failed {
		t.Error("gate should pass after clearing failure")
	}
	if _, stillFailed := s.DriveStepFailed("PROJ", "PROJ-1", stepAC); stillFailed {
		t.Error("recorded AC failure should be cleared on success")
	}
}
