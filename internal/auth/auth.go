// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package auth

import (
	"fmt"
	"os"
)

// Authenticator handles YouTube credentials (cookies).
type Authenticator interface {
	GetCookiePath() string
	SetCookiePath(path string) error
	IsAuthenticated() bool
}

// CookieAuth implements Authenticator by pointing to a Netscape cookies file.
type CookieAuth struct {
	path string
}

// NewCookieAuth creates a new CookieAuth.
func NewCookieAuth(path string) *CookieAuth {
	return &CookieAuth{path: path}
}

func (c *CookieAuth) GetCookiePath() string {
	return c.path
}

func (c *CookieAuth) SetCookiePath(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("cookie file not found: %w", err)
	}
	c.path = path
	return nil
}

func (c *CookieAuth) IsAuthenticated() bool {
	return c.path != ""
}
