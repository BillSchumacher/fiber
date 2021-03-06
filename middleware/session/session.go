package session

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

type Session struct {
	ctx    *fiber.Ctx
	config *Store
	db     *db
	id     string
	fresh  bool
}

var sessionPool = sync.Pool{
	New: func() interface{} {
		return new(Session)
	},
}

func acquireSession() *Session {
	s := sessionPool.Get().(*Session)
	s.db = new(db)
	s.fresh = true
	return s
}

func releaseSession(s *Session) {
	s.ctx = nil
	s.config = nil
	if s.db != nil {
		s.db.Reset()
	}
	s.id = ""
	s.fresh = true
	sessionPool.Put(s)
}

// Fresh is true if the current session is new
func (s *Session) Fresh() bool {
	return s.fresh
}

// ID returns the session id
func (s *Session) ID() string {
	return s.id
}

// Get will return the value
func (s *Session) Get(key string) interface{} {
	return s.db.Get(key)
}

// Set will update or create a new key value
func (s *Session) Set(key string, val interface{}) {
	s.db.Set(key, val)
}

// Delete will delete the value
func (s *Session) Delete(key string) {
	s.db.Delete(key)
}

// Destroy will delete the session from Storage and expire session cookie
func (s *Session) Destroy() error {
	// Reset local data
	s.db.Reset()

	// Delete data from storage
	if err := s.config.Storage.Delete(s.id); err != nil {
		return err
	}

	// Expire cookie
	s.delCookie()
	return nil
}

// Regenerate generates a new session id and delete the old one from Storage
func (s *Session) Regenerate() error {

	// Delete old id from storage
	if err := s.config.Storage.Delete(s.id); err != nil {
		return err
	}
	// Create new ID
	s.id = s.config.KeyGenerator()

	return nil
}

// Save will update the storage and client cookie
func (s *Session) Save() error {
	// Don't save to Storage if no data is available
	if s.db.Len() <= 0 {
		return nil
	}

	// Convert book to bytes
	data, err := s.db.MarshalMsg(nil)
	if err != nil {
		return err
	}

	// pass raw bytes with session id to provider
	if err := s.config.Storage.Set(s.id, data, s.config.Expiration); err != nil {
		return err
	}

	// Create cookie with the session ID
	s.setCookie()

	// release session to pool to be re-used on next request
	releaseSession(s)

	return nil
}

func (s *Session) setCookie() {
	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(s.config.CookieName)
	fcookie.SetValue(s.id)
	fcookie.SetPath(s.config.CookiePath)
	fcookie.SetDomain(s.config.CookieDomain)
	fcookie.SetMaxAge(int(s.config.Expiration.Seconds()))
	fcookie.SetExpire(time.Now().Add(s.config.Expiration))
	fcookie.SetSecure(s.config.CookieSecure)
	fcookie.SetHTTPOnly(s.config.CookieHTTPOnly)

	switch utils.ToLower(s.config.CookieSameSite) {
	case "strict":
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case "none":
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	}

	s.ctx.Response().Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}

func (s *Session) delCookie() {
	s.ctx.Request().Header.DelCookie(s.config.CookieName)
	s.ctx.Response().Header.DelCookie(s.config.CookieName)

	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(s.config.CookieName)
	fcookie.SetPath(s.config.CookiePath)
	fcookie.SetDomain(s.config.CookieDomain)
	fcookie.SetMaxAge(-1)
	fcookie.SetExpire(time.Now().Add(-1 * time.Minute))
	fcookie.SetSecure(s.config.CookieSecure)
	fcookie.SetHTTPOnly(s.config.CookieHTTPOnly)

	switch utils.ToLower(s.config.CookieSameSite) {
	case "strict":
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case "none":
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	}

	s.ctx.Response().Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}
