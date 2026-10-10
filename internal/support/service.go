package support

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

const (
	maxTitleLen = 120
	maxTextLen  = 4000
	// DefaultLimit and DefaultWindow: 5 reports per rolling 24 hours per account.
	DefaultLimit  = 5
	DefaultWindow = 24 * time.Hour
)

// Store is the persistence the service needs; *data.SupportReportStore.
type Store interface {
	Create(ctx context.Context, r *data.SupportReport) error
	ListByUser(ctx context.Context, userID int) ([]data.SupportReport, error)
	CountSince(ctx context.Context, userID int, since time.Time) (int, time.Time, error)
	MarkSent(ctx context.Context, id int, repo string, number int, url string) error
	MarkFailed(ctx context.Context, id int, msg string) error
	ListPending(ctx context.Context, limit int) ([]data.SupportReport, error)
	ListOpenToSync(ctx context.Context, olderThan time.Time, limit int) ([]data.SupportReport, error)
	UpdateStatus(ctx context.Context, id int, status string) error
	WithSyncLock(ctx context.Context, fn func() error) error
}

// Settings configure where issues go.
type Settings struct {
	PseudonymKey []byte
	PublicRepo   string
	PrivateRepo  string
	ProjectID    string
	Limit        int
	Window       time.Duration
}

// Service files reports and keeps them in step with GitHub. GH may be nil
// while the GitHub App is not configured: reports then stay pending.
type Service struct {
	Store    Store
	GH       GitHub
	Settings Settings
	Now      func() time.Time
}

func NewService(store Store, gh GitHub, s Settings) *Service {
	if s.Limit <= 0 {
		s.Limit = DefaultLimit
	}
	if s.Window <= 0 {
		s.Window = DefaultWindow
	}
	return &Service{Store: store, GH: gh, Settings: s, Now: time.Now}
}

// Input is a report as submitted by the form.
type Input struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Fields
}

// FieldError is a missing or invalid field (HTTP 400).
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }

// SensitiveError is a field that looks like it holds a password, a secret,
// an email or a postal address (HTTP 422). It never carries the text itself.
type SensitiveError struct{ Field, Category string }

func (e *SensitiveError) Error() string { return "sensitive content in " + e.Field + ": " + e.Category }

// RateLimitError means the account reached its quota (HTTP 429).
type RateLimitError struct{ RetryAt time.Time }

func (e *RateLimitError) Error() string { return "support report limit reached" }

var allowed = map[string]map[string]bool{
	"app":    {"web": true, "ios": true, "android": true, "gateway": true, "other": true},
	"mode":   {"cloud": true, "lan": true, "unknown": true},
	"impact": {"module": true, "local": true, "remote": true, "hardware": true, "unknown": true},
}

func (in *Input) validate() error {
	in.Title = strings.TrimSpace(in.Title)
	if in.Kind != KindBug && in.Kind != KindIncident {
		return &FieldError{"kind", "invalid"}
	}
	if in.Title == "" {
		return &FieldError{"title", "required"}
	}
	if utf8.RuneCountInString(in.Title) > maxTitleLen {
		return &FieldError{"title", "too_long"}
	}
	if strings.TrimSpace(in.Actual) == "" {
		return &FieldError{"actual", "required"}
	}
	closed := map[string]string{"app": in.App, "mode": in.Mode, "impact": in.Impact}
	for name, v := range closed {
		if v != "" && !allowed[name][v] {
			return &FieldError{name, "invalid"}
		}
	}
	if in.Kind == KindBug && in.App == "" {
		return &FieldError{"app", "required"}
	}
	if in.Kind == KindIncident && in.Impact == "" {
		return &FieldError{"impact", "required"}
	}
	free := []struct{ name, v string }{
		{"version", in.Version}, {"since", in.Since}, {"actual", in.Actual},
		{"expected", in.Expected}, {"steps", in.Steps}, {"tried", in.Tried},
	}
	for _, f := range free {
		if utf8.RuneCountInString(f.v) > maxTextLen {
			return &FieldError{f.name, "too_long"}
		}
	}
	for _, f := range append([]struct{ name, v string }{{"title", in.Title}}, free...) {
		if cat := DetectSensitive(f.v); cat != "" {
			return &SensitiveError{f.name, cat}
		}
	}
	return nil
}

// Submit validates, rate-limits and stores the report, then tries to open the
// GitHub issue at once. A GitHub failure is not the user's problem: the
// report stays pending and the syncer retries.
func (s *Service) Submit(ctx context.Context, user *domain.User, in Input) (*data.SupportReport, error) {
	if err := in.validate(); err != nil {
		if se, ok := err.(*SensitiveError); ok {
			log.Printf("audit action=SUPPORT_REPORT_REJECTED user_id=%d field=%s category=%s", user.ID, se.Field, se.Category)
		}
		return nil, err
	}
	since := s.Now().Add(-s.Settings.Window)
	n, oldest, err := s.Store.CountSince(ctx, user.ID, since)
	if err != nil {
		return nil, err
	}
	if n >= s.Settings.Limit {
		log.Printf("audit action=SUPPORT_REPORT_BLOCKED_RATELIMIT user_id=%d", user.ID)
		return nil, &RateLimitError{RetryAt: oldest.Add(s.Settings.Window)}
	}
	ref, err := NewReportRef()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(in.Fields)
	if err != nil {
		return nil, err
	}
	rep := &data.SupportReport{PublicRef: ref, UserID: user.ID, Kind: in.Kind, Title: in.Title, Payload: payload}
	if err := s.Store.Create(ctx, rep); err != nil {
		return nil, err
	}
	log.Printf("audit action=SUPPORT_REPORT_CREATED user_id=%d ref=%s kind=%s", user.ID, ref, in.Kind)
	if s.GH != nil {
		if err := s.send(ctx, rep); err != nil {
			log.Printf("support: send %s deferred: %v", ref, err)
		}
	}
	return rep, nil
}

// List returns the account's reports.
func (s *Service) List(ctx context.Context, user *domain.User) ([]data.SupportReport, error) {
	return s.Store.ListByUser(ctx, user.ID)
}

func (s *Service) repoFor(kind string) string {
	if kind == KindIncident {
		return s.Settings.PrivateRepo
	}
	return s.Settings.PublicRepo
}

// send opens the issue for a stored report and records it.
func (s *Service) send(ctx context.Context, rep *data.SupportReport) error {
	var f Fields
	if err := json.Unmarshal(rep.Payload, &f); err != nil {
		_ = s.Store.MarkFailed(ctx, rep.ID, "payload: unreadable")
		return err
	}
	in := IssueInput{
		Title:  IssueTitle(rep.Kind, rep.Title),
		Body:   IssueBody(rep.Kind, f, UserPseudonym(s.Settings.PseudonymKey, rep.UserID), rep.PublicRef),
		Labels: []string{"bug", "via-portal"},
		Type:   "Bug",
	}
	if rep.Kind == KindIncident {
		in.Labels, in.Type = []string{"incident", "via-portal"}, "Support"
	}
	repo := s.repoFor(rep.Kind)
	iss, err := s.GH.CreateIssue(ctx, repo, in)
	if err != nil {
		_ = s.Store.MarkFailed(ctx, rep.ID, errorSummary(err))
		return err
	}
	if s.Settings.ProjectID != "" && iss.NodeID != "" {
		if err := s.GH.AddToProject(ctx, s.Settings.ProjectID, iss.NodeID); err != nil {
			log.Printf("support: add %s to project: %v", rep.PublicRef, err)
		}
	}
	if err := s.Store.MarkSent(ctx, rep.ID, repo, iss.Number, iss.HTMLURL); err != nil {
		return err
	}
	rep.Status, rep.Repo.String, rep.Repo.Valid = data.SupportStatusOpen, repo, true
	rep.IssueURL.String, rep.IssueURL.Valid = iss.HTMLURL, true
	return nil
}

func errorSummary(err error) string {
	var e *APIError
	if errors.As(err, &e) {
		return e.Error()
	}
	return "transport error"
}

// StatusFromIssue maps a GitHub issue state onto a report status.
func StatusFromIssue(iss Issue) string {
	if iss.State != "closed" {
		return data.SupportStatusOpen
	}
	if iss.StateReason == "completed" {
		return data.SupportStatusResolved
	}
	return data.SupportStatusClosed
}
