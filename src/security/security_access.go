package security

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jotitan/photos_server/config"
	"github.com/jotitan/photos_server/logger"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type JWTManager interface {
	getJWT(r *http.Request) (*jwt.Token, error)
	canSign() bool
	getSigningKey() []byte
}

func NewJWTManager(conf config.SecurityConfig) (JWTManager, error) {
	switch {
	case conf.UrlPublicKeys != "":
		return newSecurity(conf.UrlPublicKeys), nil
	case conf.HS256SecretKey != "":
		logger.GetLogger2().Info("Use JWT security mode")
		return JWTSecretKeyManager{[]byte(conf.HS256SecretKey)}, nil
	default:
		logger.GetLogger2().Info("No security mode, stop")
		return nil, errors.New("specify a jwt token security mode")
	}
}

type JWTSecretKeyManager struct {
	hs256SecretKey []byte
}

func (jwts JWTSecretKeyManager) getJWT(r *http.Request) (*jwt.Token, error) {
	c, err := r.Cookie("token")
	if err != nil {
		return nil, errors.New("impossible to get jwt")
	}
	return jwt.Parse(c.Value, func(token *jwt.Token) (interface{}, error) { return jwts.hs256SecretKey, nil })
}

func (jwts JWTSecretKeyManager) getSigningKey() []byte {
	return jwts.hs256SecretKey
}

func (jwts JWTSecretKeyManager) canSign() bool {
	return true
}

type JWTIdentityKeysManager struct {
	publicKeysCache map[string]*rsa.PublicKey
	urlGetPublicKey string
}

func newSecurity(url string) JWTIdentityKeysManager {
	return JWTIdentityKeysManager{make(map[string]*rsa.PublicKey), url}
}

func (s JWTIdentityKeysManager) canSign() bool {
	return false
}

func (s JWTIdentityKeysManager) getSigningKey() []byte {
	return nil
}

func (s JWTIdentityKeysManager) getJWT(r *http.Request) (*jwt.Token, error) {
	c, err := r.Cookie("token")
	if err != nil {
		return nil, errors.New("impossible to get jwt")
	}
	return jwt.Parse(c.Value, func(t *jwt.Token) (interface{}, error) {
		kid, exist := t.Header["kid"]
		if !exist {
			return nil, errors.New("missing kid in geader")
		}
		return s.findPublicKey(kid.(string))
	})
}

func (s *JWTIdentityKeysManager) findPublicKey(kid string) (*rsa.PublicKey, error) {
	key, exist := s.publicKeysCache[kid]
	if !exist {
		var err error
		key, err = s.getPublicKey(kid)
		if err != nil {
			return nil, err
		}
		s.publicKeysCache[kid] = key
	}
	return key, nil
}

func (s *JWTIdentityKeysManager) getPublicKey(kid string) (*rsa.PublicKey, error) {

	resp, err := http.Get(fmt.Sprintf("%s?kid=%s", s.urlGetPublicKey, url.QueryEscape(kid)))
	if err != nil {
		return nil, err
	}
	if data, err := io.ReadAll(resp.Body); err == nil {
		return jwt.ParseRSAPublicKeyFromPEM(data)
	} else {
		return nil, err
	}
}

type SecurityAccess struct {
	maskForAdmin string
	// true only if username and password exist
	userAccessEnable bool
	// Used for jwt token
	jwtManager     JWTManager
	accessProvider AccessProvider
	// Store shares with other people
	ShareFolders *ShareFolders
}

func NewSecurityAccess(conf config.SecurityConfig, maskForAdmin string, hs256SecretKey []byte) *SecurityAccess {
	sa := SecurityAccess{maskForAdmin: maskForAdmin, userAccessEnable: false}
	if jwtManager, err := NewJWTManager(conf); err == nil {
		sa.jwtManager = jwtManager
	} else {
		logger.GetLogger2().Info("Use simple security mode")
	}
	sa.ShareFolders = NewShareFolders(&sa)
	return &sa
}

func (sa SecurityAccess) checkMaskAccess(r *http.Request) bool {
	return !strings.EqualFold("", sa.maskForAdmin) && (strings.Contains(r.Referer(), sa.maskForAdmin) ||
		strings.Contains(r.RemoteAddr, sa.maskForAdmin))
}

func (sa SecurityAccess) getJWTCookie(r *http.Request) *http.Cookie {
	if c, err := r.Cookie("token"); err == nil {
		return c
	}
	return nil
}

func (sa SecurityAccess) checkAccess(r *http.Request) bool {
	// Two cases : on local network or with basic authent
	if sa.checkMaskAccess(r) {
		return true
	}
	return false
}

func (sa SecurityAccess) getJWT(r *http.Request) (*jwt.Token, error) {
	return sa.jwtManager.getJWT(r)
}

func (sa SecurityAccess) GetUserId(r *http.Request) string {
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if jwtToken, err := sa.getJWT(r); err == nil {
		return sa.accessProvider.GetId(jwtToken)
	}
	return ""
}

// Check read access, for regular user or guest
func (sa SecurityAccess) CheckJWTTokenAccess(r *http.Request) bool {
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if jwtToken, err := sa.getJWT(r); err == nil {
		return sa.accessProvider.CheckReadAccess(jwtToken)
	}
	return false
}

func (sa SecurityAccess) CheckJWTTokenRegularAccess(r *http.Request) bool {
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if jwtToken, err := sa.getJWT(r); err == nil {
		return sa.accessProvider.CheckRegularReadAccess(jwtToken)
	}
	return false
}

// Check write access
func (sa SecurityAccess) CheckJWTTokenAdminAccess(r *http.Request) bool {
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if jwtToken, err := sa.getJWT(r); err == nil {
		return sa.accessProvider.CheckAdminAccess(jwtToken)
	}
	return false
}

func (sa SecurityAccess) setJWTToken(extras map[string]interface{}, w http.ResponseWriter) {
	// Only if security key can manage it
	if sa.jwtManager.canSign() {
		// No expiracy
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(extras))
		if sToken, err := token.SignedString(sa.jwtManager.getSigningKey()); err == nil {
			cookie := http.Cookie{}
			cookie.Name = "token"
			cookie.Path = "/"
			cookie.Value = sToken
			cookie.HttpOnly = true
			cookie.SameSite = http.SameSiteLaxMode
			http.SetCookie(w, &cookie)
		}
	}
}

// Return security type with parameters
func (sa SecurityAccess) GetTypeSecurity() string {
	return sa.accessProvider.Info()
}

func (sa SecurityAccess) IsGuest(r *http.Request) bool {
	if token, err := sa.getJWT(r); err == nil {
		return sa.accessProvider.CheckGuestAccess(token)
	}
	return false
}

// Try connect by provider with parameters
func (sa SecurityAccess) Connect(w http.ResponseWriter, r *http.Request) bool {
	// Check if jwt token exist and valid. Create by server during first connexion
	if sa.getJWTCookie(r) != nil {
		// If jwt already exist, don't create a new
		return true
	}

	if !sa.accessProvider.CanConnect() {
		return false
	}
	if success, extras := sa.accessProvider.Connect(r, sa.ShareFolders.Connect); success {
		sa.setJWTToken(extras, w)
		return true
	}
	return false
}

func (sa *SecurityAccess) SetAccessProvider(provider AccessProvider) {
	sa.accessProvider = provider
}
