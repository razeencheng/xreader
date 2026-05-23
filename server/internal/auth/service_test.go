package auth

import (
	"context"
	"testing"

	"github.com/razeencheng/xreader/internal/testutil"
	"github.com/stretchr/testify/require"
)

type mockGitHub struct {
	user *GitHubUser
	err  error
}

func (m *mockGitHub) AuthCodeURL(state string) string {
	return "https://github.com/login/oauth/authorize?state=" + state
}

func (m *mockGitHub) ExchangeCode(_ context.Context, _ string) (string, error) {
	return "mock-token", m.err
}

func (m *mockGitHub) FetchUser(_ context.Context, _ string) (*GitHubUser, error) {
	return m.user, m.err
}

type mockAllowlist struct {
	allowed map[string]bool
}

func (m *mockAllowlist) IsAllowlisted(_ context.Context, username string) (bool, error) {
	return m.allowed[username], nil
}

type mockUserStore struct {
	nextID int64
}

func (m *mockUserStore) UpsertUser(_ context.Context, _ int64, _, _ string) (int64, error) {
	m.nextID++
	return m.nextID, nil
}

type mockSessionCreator struct{}

func (m *mockSessionCreator) Create(_ context.Context, _ int64, _ string) (string, error) {
	return "session-123", nil
}

var testCookieState = NewCookieState([]byte("test-secret-key"))

func newTestService(ghUser *GitHubUser, allowedUsers []string) *Service {
	allowed := make(map[string]bool)
	for _, u := range allowedUsers {
		allowed[u] = true
	}
	return &Service{
		GitHub:      &mockGitHub{user: ghUser},
		CookieState: testCookieState,
		Allowlist:   &mockAllowlist{allowed: allowed},
		Users:       &mockUserStore{},
		Sessions:    &mockSessionCreator{},
	}
}

func TestAuthService_Callback_DeniesUnallowlistedUser(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 123, Username: "stranger"},
		[]string{"alice", "bob"},
	)

	// Generate a valid state token.
	state, err := svc.CookieState.Generate()
	require.NoError(t, err)

	_, err = svc.Callback(context.Background(), state, state, "gh-code", "test-agent")
	require.ErrorIs(t, err, ErrNotAllowlisted)
}

func TestAuthService_Callback_HappyPath_CreatesUserAndSession(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 456, Username: "alice", AvatarURL: "https://avatar"},
		[]string{"alice"},
	)

	state, err := svc.CookieState.Generate()
	require.NoError(t, err)

	result, err := svc.Callback(context.Background(), state, state, "gh-code", "test-agent")
	require.NoError(t, err)
	require.Equal(t, "session-123", result.SessionID)
	require.Greater(t, result.UserID, int64(0))
}

func TestAuthService_Callback_RejectsInvalidState(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 123, Username: "alice"},
		[]string{"alice"},
	)

	_, err := svc.Callback(context.Background(), "bad-state", "bad-state", "gh-code", "test-agent")
	require.ErrorIs(t, err, ErrInvalidState)
}

func TestAuthService_Callback_RejectsMismatchedState(t *testing.T) {
	svc := newTestService(
		&GitHubUser{GitHubID: 123, Username: "alice"},
		[]string{"alice"},
	)

	state1, err := svc.CookieState.Generate()
	require.NoError(t, err)
	state2, err := svc.CookieState.Generate()
	require.NoError(t, err)

	// stateParam != cookieValue → rejected
	_, err = svc.Callback(context.Background(), state1, state2, "gh-code", "test-agent")
	require.ErrorIs(t, err, ErrInvalidState)
}

func TestAuthService_Callback_SeededAdminFirstLoginGetsAdminRole(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := testutil.SetupTestDB(t, ctx)
	t.Cleanup(cleanup)

	_, err := pool.Exec(ctx,
		"INSERT INTO auth_allowlist (github_username, note) VALUES ($1, $2)",
		"razeencheng", "seed-admin CLI",
	)
	require.NoError(t, err)

	cs := NewCookieState([]byte("test-secret"))
	state, err := cs.Generate()
	require.NoError(t, err)

	svc := &Service{
		GitHub: &mockGitHub{
			user: &GitHubUser{
				GitHubID:  456,
				Username:  "razeencheng",
				AvatarURL: "https://avatar",
			},
		},
		CookieState: cs,
		Allowlist:   &mockAllowlist{allowed: map[string]bool{"razeencheng": true}},
		Users:       NewPgUserStore(pool),
		Sessions:    &mockSessionCreator{},
	}

	result, err := svc.Callback(ctx, state, state, "gh-code", "test-agent")
	require.NoError(t, err)
	require.NotZero(t, result.UserID)

	var role string
	err = pool.QueryRow(ctx, "SELECT role FROM users WHERE id = $1", result.UserID).Scan(&role)
	require.NoError(t, err)
	require.Equal(t, "admin", role)
}
