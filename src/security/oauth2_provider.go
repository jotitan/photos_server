package security

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-errors/errors"
	"github.com/jotitan/photos_server/config"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// Give oauth2 provider to let acccess to application

// Differents provider could be implemented (google, azure...)
type Provider interface{
	//GenerateUrl generate an url to access application from oauth2 access
	GenerateUrl(pathFolder,email string)string
	GenerateUrlConnection()string
	// Get a valid jwt token from oauth2 provider from code
	GetTokenFromCode(code string)(string,error)
	// Extract data from jwt token
	CheckAndExtractData(token string)(map[string]interface{},error)
	CheckEmail(email string) bool
}

func NewProvider(conf config.OAuth2Config)Provider{
	switch conf.Provider {
	case "google":return NewGoogleProvider(conf.ClientID,conf.ClientSecret,conf.RedirectUrl)
	default:return nil
	}
}

type GoogleProvider struct{
	clientID string
	clientSecret string
	urlGenerateCode string
	redirectUrl string
	urlToken string
}

func NewGoogleProvider(clientID,clientSecret,redirectUrl string)GoogleProvider{
	return GoogleProvider{
		clientID:clientID,
		clientSecret:clientSecret,
		redirectUrl:redirectUrl,
		urlGenerateCode:"https://accounts.google.com/o/oauth2/v2/auth",
		urlToken:"https://oauth2.googleapis.com/token",
	}
}

func (gp GoogleProvider) CheckEmail(email string) bool {
	return strings.HasSuffix(email,"@gmail.com")
}

func (gp GoogleProvider) GenerateUrlConnection() string {
	return fmt.Sprintf("%s?scope=%s&client_id=%s&redirect_uri=%s&response_type=code&flowName=GeneralOAuthFlow",
		gp.urlGenerateCode,"https://www.googleapis.com/auth/userinfo.email",gp.clientID,gp.redirectUrl)
}

func (gp GoogleProvider) CheckAndExtractData(token string) (map[string]interface{}, error) {
	// No need to use a valid signature (token send by google, must be trust
	if token,_ := jwt.Parse(token,func(token *jwt.Token)(interface{}, error){return nil,nil}) ; token == nil {
		//error
		return nil,errors.New("impossible to get informations from google jwt token")
	}else{
		claims := token.Claims.(jwt.MapClaims)
		if !strings.EqualFold(gp.clientID,claims["aud"].(string)) {
			return nil,errors.New("bad jwt token")
		}
		if ! claims["email_verified"].(bool) {
			return nil,errors.New("email not verified")
		}
		m := make(map[string]interface{})
		m["email"] = claims["email"].(string)
		return m,nil
	}
}

func (gp GoogleProvider)GenerateUrl(pathFolder,email string)string{
	return fmt.Sprintf("%s?scope=%s&client_id=%s&redirect_uri=%s&response_type=code&state=%s&flowName=GeneralOAuthFlow",
		"https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fuserinfo.email",gp.urlGenerateCode,gp.clientID,gp.redirectUrl,pathFolder)
}

// Get a JWT token on google oauth2 from code
func (gp GoogleProvider)GetTokenFromCode(code string)(string,error){
	urlGetToken := fmt.Sprintf("%s?client_id=%s&client_secret=%s&code=%s&redirect_uri=%s&grant_type=authorization_code",gp.urlToken,gp.clientID,gp.clientSecret,code,gp.redirectUrl)

	if resp,err := http.PostForm(urlGetToken,url.Values{}) ; err == nil && resp.StatusCode == 200{
		if data,err := ioutil.ReadAll(resp.Body) ; err == nil {
			m := make(map[string]interface{})
			if err := json.Unmarshal(data,&m) ; err == nil {
				return m["id_token"].(string),nil
			}else{
				return "",err
			}
		}else{
			return "",err
		}
	}else{
		if err == nil {
			return "",errors.New("no token send by google provider")
		}
		return "",err
	}
}