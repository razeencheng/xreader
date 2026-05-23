package auth

import (
	"context"
	"errors"
)

var (
	ErrNotAllowlisted = errors.New("user not on allowlist")
	ErrInvalidState   = errors.New("invalid or expired CSRF state")
)

type GitHubUser struct {
	GitHubID  int64
	Username  string
	AvatarURL string
}

type GitHubClient interface {
	AuthCodeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (token string, err error)
	FetchUser(ctx context.Context, token string) (*GitHubUser, error)
}

type AllowlistChecker interface {
	IsAllowlisted(ctx context.Context, username string) (bool, error)
}

type UserStore interface {
	UpsertUser(ctx context.Context, githubID int64, username, avatarURL string) (userID int64, err error)
}

type SessionCreator interface {
	Create(ctx context.Context, userID int64, userAgent string) (sessionID string, err error)
}

type Service struct {
	GitHub      GitHubClient
	CookieState *CookieState
	Allowlist   AllowlistChecker
	Users       UserStore
	Sessions    SessionCreator
}

type CallbackResult struct {
	SessionID string
	UserID    int64
}

// BeginLogin generates a signed CSRF state token and returns the GitHub
// redirect URL along with the raw state value (so the handler can set it
// as a cookie).
func (s *Service) BeginLogin() (redirectURL, state string, err error) {
	state, err = s.CookieState.Generate()
	if err != nil {
		return "", "", err
	}
	return s.GitHub.AuthCodeURL(state), state, nil
}

// Callback verifies the OAuth CSRF state by comparing the query-string
// state parameter with the cookie value, then completes the login flow.
func (s *Service) Callback(ctx context.Context, stateParam, cookieValue, code, userAgent string) (*CallbackResult, error) {
	if !s.CookieState.Verify(stateParam, cookieValue) {
		return nil, ErrInvalidState
	}

	token, err := s.GitHub.ExchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}

	ghUser, err := s.GitHub.FetchUser(ctx, token)
	if err != nil {
		return nil, err
	}

	allowed, err := s.Allowlist.IsAllowlisted(ctx, ghUser.Username)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotAllowlisted
	}

	userID, err := s.Users.UpsertUser(ctx, ghUser.GitHubID, ghUser.Username, ghUser.AvatarURL)
	if err != nil {
		return nil, err
	}

	sessionID, err := s.Sessions.Create(ctx, userID, userAgent)
	if err != nil {
		return nil, err
	}

	return &CallbackResult{SessionID: sessionID, UserID: userID}, nil
}
