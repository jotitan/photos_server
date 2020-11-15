package security

import (
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/jotitan/photos_server/logger"
	"net/http"
	"strings"
)

type SecurityAccess struct {
	maskForAdmin string
	// true only if username and password exist
	userAccessEnable bool
	// Used for jwt token
	hs256SecretKey   []byte
	accessProvider  AccessProvider
	// Store shares with other people
	ShareFolders * ShareFolders
}

func NewSecurityAccess(maskForAdmin string,hs256SecretKey []byte)*SecurityAccess{
	sa := SecurityAccess{maskForAdmin:maskForAdmin,userAccessEnable:false}
	if len(hs256SecretKey) > 0 {
		sa.hs256SecretKey = hs256SecretKey
		logger.GetLogger2().Info("Use JWT security mode")
	}else{
		logger.GetLogger2().Info("Use simple security mode")
	}
	sa.ShareFolders = NewShareFolders(&sa)
	return &sa
}

func (sa SecurityAccess)checkMaskAccess(r * http.Request)bool{
	return !strings.EqualFold("",sa.maskForAdmin) && (
		strings.Contains(r.Referer(),sa.maskForAdmin) ||
			strings.Contains(r.RemoteAddr,sa.maskForAdmin))
}

func (sa SecurityAccess)getJWTCookie(r * http.Request)*http.Cookie{
	for _,c := range r.Cookies() {
		if strings.EqualFold("token",c.Name) {
			return c
		}
	}
	return nil
}

func (sa SecurityAccess)checkAccess(r * http.Request)bool{
	// Two cases : on local network or with basic authent
	if sa.checkMaskAccess(r) {
		return true
	}
	return false
}

func (sa SecurityAccess) getJWT(r * http.Request)(*jwt.Token,error){
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if token := sa.getJWTCookie(r); token != nil {
		return jwt.Parse(token.Value,func(token *jwt.Token) (interface{}, error) {return sa.hs256SecretKey,nil})
	}
	return nil,errors.New("impossible to get jwt")
}

func (sa SecurityAccess) GetUserId(r * http.Request)string{
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if jwtToken,err := sa.getJWT(r) ; err == nil{
		return sa.accessProvider.GetId(jwtToken)
	}
	return ""
}

// Check read access, for regular user or guest
func (sa SecurityAccess) CheckJWTTokenAccess(r * http.Request)bool{
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if jwtToken,err := sa.getJWT(r) ; err == nil{
		return sa.accessProvider.CheckReadAccess(jwtToken)
	}
	return false
}

// Check write access
func (sa SecurityAccess) CheckJWTTokenAdminAccess(r * http.Request)bool{
	// Check if jwt token exist in a cookie and is valid. Create by server during first connexion
	if jwtToken,err := sa.getJWT(r) ; err == nil{
		return sa.accessProvider.CheckAdminAccess(jwtToken)
	}
	return false
}

func (sa SecurityAccess)setJWTToken(extras map[string]interface{},w http.ResponseWriter){
	// No expiracy
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,jwt.MapClaims(extras))
	if sToken,err := token.SignedString(sa.hs256SecretKey); err == nil {
		cookie := http.Cookie{}
		cookie.Name="token"
		cookie.Path="/"
		cookie.Value=sToken
		cookie.HttpOnly=true
		cookie.SameSite=http.SameSiteLaxMode
		http.SetCookie(w,&cookie)
	}
}

// Return security type with parameters
func (sa SecurityAccess) GetTypeSecurity()string{
	return sa.accessProvider.Info()
}

func (sa SecurityAccess) IsGuest(r *http.Request)bool{
	if token,err := sa.getJWT(r) ; err == nil {
		return sa.accessProvider.CheckGuestAccess(token)
	}
	return false
}

// Try connect by provider with parameters
func (sa SecurityAccess) Connect(w http.ResponseWriter,r * http.Request)bool{
	// Check if jwt token exist and valid. Create by server during first connexion
	if sa.getJWTCookie(r) != nil{
		// If jwt already exist, don't create a new
		return true
	}
	if success,extras := sa.accessProvider.Connect(r,sa.ShareFolders.Connect) ; success{
		sa.setJWTToken(extras,w)
		return true
	}
	return false
}

func (sa * SecurityAccess)SetAccessProvider(provider AccessProvider){
	sa.accessProvider = provider
}