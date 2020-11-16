package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jotitan/photos_server/logger"
	"io/ioutil"
	"net/http"
	"strings"
)

// Different security provider : basic, oauth2

type AccessProvider interface{
	// Connect with provider. Return success and if true, list of additional parameter (for jwt token)
	Connect(r * http.Request,isGuest func(user string)bool)(bool,map[string]interface{})
	CheckReadAccess(token *jwt.Token)bool
	CheckRegularReadAccess(token *jwt.Token)bool
	CheckGuestAccess(token *jwt.Token)bool
	GetId(token *jwt.Token)string
	CheckAdminAccess(token *jwt.Token) bool
	Info()string
	// Check if a mail can be used for sharing
	CheckShareMailValid(email string)bool
}

type OAuth2AccessProvider struct {
	provider Provider
	// Authorized emails
	authorizedEmails	map[string]struct{}
	adminEmails	map[string]struct{}
}

func extractCode(r * http.Request)(string,error){
	if d,err := ioutil.ReadAll(r.Body) ; err == nil {
		params := make([]map[string]interface{},0)
		if err := json.Unmarshal(d,&params) ; err != nil {
			return "",err
		}
		for _,param := range params {
			if strings.EqualFold("code",param["name"].(string)){
				return param["value"].(string),nil
			}
		}
		return "",errors.New("no code param")
	}else{
		return "",err
	}

}

func (oauth2ap OAuth2AccessProvider) CheckShareMailValid(email string) bool {
	return oauth2ap.provider.CheckEmail(email)
}

func (oauth2ap OAuth2AccessProvider) Connect(r *http.Request,isGuest func(user string)bool) (bool, map[string]interface{}) {
	// Extract parameter code
	if code,err := extractCode(r) ; err == nil {
		if token, err := oauth2ap.provider.GetTokenFromCode(code); err == nil {
			if data, err := oauth2ap.provider.CheckAndExtractData(token); err == nil {
				// Check if email exist in admin authorized list
				email := data["email"].(string)
				// Normal user
				if _,isUser := oauth2ap.authorizedEmails[email];isUser {
					if _, isAdmin := oauth2ap.adminEmails[email]; isAdmin {
						// Add is_admin
						data["is_admin"] = true
					}
					return true,data
				}else{
					// Check if user who want to connect is a guest (has some shares)
					if isGuest(email) {
						data["guest"] = true
						return true,data
					}
				}
			}
		}
	}else{
		return false,nil
	}
	return false,nil
}

func NewOAuth2AccessProvider(provider Provider,emails,emailsAdmin []string)OAuth2AccessProvider{
	mapEmails := make(map[string]struct{},len(emails))
	for _,email := range emails {
		mapEmails[email] = struct{}{}
	}
	mapEmailsAdmin := make(map[string]struct{},len(emailsAdmin))
	for _,email := range emailsAdmin {
		mapEmailsAdmin[email] = struct{}{}
	}
	logger.GetLogger2().Info("Use oauth2 access provider")
	return OAuth2AccessProvider{provider:provider,authorizedEmails:mapEmails,adminEmails:mapEmailsAdmin}
}

func (oauth2ap OAuth2AccessProvider) Info() string {
	return fmt.Sprintf("{\"name\":\"oauth2\",\"url\":\"%s\"}",oauth2ap.provider.GenerateUrlConnection())
}

func (oauth2ap OAuth2AccessProvider) CheckAdminAccess(token *jwt.Token) bool {
	if !oauth2ap.CheckReadAccess(token) {
		return false
	}
	if isAdmin,exist := token.Claims.(jwt.MapClaims)["is_admin"] ; exist {
		return isAdmin.(bool)
	}
	return false
}

func (oauth2ap OAuth2AccessProvider)CheckReadAccess(token * jwt.Token)bool{
	email := token.Claims.(jwt.MapClaims)["email"].(string)
	if _,exist := oauth2ap.authorizedEmails[email] ; exist {
		return true
	}
	return oauth2ap.CheckGuestAccess(token)
}

func (oauth2ap OAuth2AccessProvider)CheckRegularReadAccess(token * jwt.Token)bool{
	email := token.Claims.(jwt.MapClaims)["email"].(string)
	if _,exist := oauth2ap.authorizedEmails[email] ; exist {
		return true
	}
	return false
}

func (oauth2ap OAuth2AccessProvider)CheckGuestAccess(token * jwt.Token)bool{
	// Check if a share exist
	if isGuest,exist := token.Claims.(jwt.MapClaims)["guest"];exist {
		return isGuest.(bool)
	}
	return false
}

func (oauth2ap OAuth2AccessProvider)GetId(token * jwt.Token)string{
	return token.Claims.(jwt.MapClaims)["email"].(string)
}

type BasicProvider struct {
	username string
	password string
}

func (bp BasicProvider) CheckShareMailValid(email string) bool {
	return true
}

func (bp BasicProvider) Connect(r *http.Request,isGuest func(user string)bool) (bool, map[string]interface{}) {
	if username, password, ok := r.BasicAuth(); ok && !strings.EqualFold("", username) {
		success :=  strings.EqualFold(username, bp.username) && strings.EqualFold(password, bp.password)
		extras := make(map[string]interface{})
		if success {
			logger.GetLogger2().Info(fmt.Sprintf("User %s connected",username))
			extras["is_admin"] = true
			extras["username"] = username
		}else{
			logger.GetLogger2().Error(fmt.Sprintf("User %s try to connect but fail",username))
		}
		return success,extras
	}
	return false,map[string]interface{}{}
}

func NewBasicProvider(username,password string)BasicProvider{
	logger.GetLogger2().Info("Use basic access provider")
	return BasicProvider{username,password}
}

func (bp BasicProvider) Info() string {
	return "{\"name\":\"basic\"}"
}

func (bp BasicProvider) CheckAdminAccess(token *jwt.Token) bool {
	if !bp.CheckReadAccess(token) {
		return false
	}
	if isAdmin,exist := token.Claims.(jwt.MapClaims)["is_admin"] ; exist {
		return isAdmin.(bool)
	}
	return false
}

func (bp BasicProvider)CheckRegularReadAccess(token * jwt.Token)bool{
	if strings.EqualFold(bp.username,token.Claims.(jwt.MapClaims)["username"].(string)) {
		return true
	}
	return false
}

func (bp BasicProvider)CheckReadAccess(token * jwt.Token)bool{
	if strings.EqualFold(bp.username,token.Claims.(jwt.MapClaims)["username"].(string)) {
		return true
	}
	return false
}

func (bp BasicProvider)CheckGuestAccess(token * jwt.Token)bool {
	return false
}

func (bp BasicProvider)GetId(token * jwt.Token)string{
	return token.Claims.(jwt.MapClaims)["username"].(string)
}