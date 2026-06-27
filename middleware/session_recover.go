package middleware

import (
	"log"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// RecoverBadSession clears an unreadable/corrupted/stale session cookie instead
// of letting an invalid browser cookie keep poisoning subsequent API requests.
// It is intentionally conservative: a valid logged-in session with "id" is kept.
func RecoverBadSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawCookie, cookieErr := c.Request.Cookie("session")
		hasSessionCookie := cookieErr == nil && rawCookie != nil && rawCookie.Value != ""

		session := sessions.Default(c)

		// Force lazy decode before downstream handlers use session.
		id := session.Get("id")
		oauthState := session.Get("oauth_state")

		// If browser sends a session cookie but the app cannot recover a user id,
		// rewrite it to a clean empty session. Preserve in-progress OAuth sessions:
		// they intentionally have no user id until the provider callback succeeds.
		if hasSessionCookie && id == nil && oauthState == nil {
			log.Printf("[sessions] clearing stale or unreadable session cookie")
			session.Clear()
			_ = session.Save()
		}

		c.Next()

		for _, err := range c.Errors {
			msg := err.Error()
			if strings.Contains(msg, "securecookie") ||
				strings.Contains(msg, "base64 decode failed") ||
				strings.Contains(msg, "the value is not valid") {
				log.Printf("[sessions] corrupted session detected, clearing cookie: %v", err)
				session.Clear()
				_ = session.Save()
				break
			}
		}
	}
}
