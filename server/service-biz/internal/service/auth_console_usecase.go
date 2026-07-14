package service

import (
	"context"
)

func (s AuthConsoleKeyService) CreateConsoleLoginKey(ctx context.Context, input CreateConsoleLoginKeyInput) (ConsoleLoginKeyView, error) {
	input = normalizeCreateConsoleLoginKeyInput(input)
	if input.UserID == "" {
		return ConsoleLoginKeyView{}, ErrInvalidArgument
	}
	if _, err := requireAuthUser(ctx, s.Users, input.UserID); err != nil {
		return ConsoleLoginKeyView{}, err
	}
	item, err := newConsoleLoginKey(s.NewSessID, authNow(s.Now), input.UserID)
	if err != nil {
		return ConsoleLoginKeyView{}, err
	}
	if err := s.Sessions.SaveConsoleLoginKey(ctx, item); err != nil {
		return ConsoleLoginKeyView{}, err
	}
	return consoleLoginKeyView(item), nil
}

func (s AuthConsoleLoginService) ConsoleLogin(ctx context.Context, input ConsoleLoginInput) (AuthSessionView, error) {
	input = normalizeConsoleLoginInput(input)
	if input.Key == "" {
		input.Key = input.LoginKey
	}
	if input.Key == "" {
		return AuthSessionView{}, ErrInvalidArgument
	}
	now := authNow(s.Now)
	item, err := requireActiveConsoleLoginKey(ctx, s.Sessions, input.Key, now.Unix())
	if err != nil {
		return AuthSessionView{}, err
	}
	user, err := requireAuthUser(ctx, s.Users, item.UserID)
	if err != nil {
		return AuthSessionView{}, err
	}
	session, err := newAuthUserSession(now, s.NewSessID, user.UserID, tokenModeShort)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := replaceUserSession(ctx, s.Sessions, session); err != nil {
		return AuthSessionView{}, err
	}
	item = markConsoleLoginKeyUsed(item, now.Unix())
	if err := s.Sessions.SaveConsoleLoginKey(ctx, item); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}
