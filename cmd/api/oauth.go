package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mayura-andrew/SpotLinkIO-backend/internal/data"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (app *application) initGoogleOAuth() {
	app.googleOauthConfig = &oauth2.Config{
		RedirectURL:  app.config.oauth.redirectURI,
		ClientID:     app.config.oauth.googleClientID,
		ClientSecret: app.config.oauth.googleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

func (app *application) googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := generateStateOauthCookie(w)

	app.logger.PrintInfo("Setting oauthState cookie", map[string]string{
		"state": state,
	})
	url := app.googleOauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
func (app *application) googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	// Verify state token
	cookie, err := r.Cookie("oauthstate")

	if err != nil {
		app.logger.PrintInfo("Missing cookie", map[string]string{
			"received_state": state,
			"error":          err.Error(),
		})
		app.invalidCredentialsResponse(w, r)
		return
	}

	if cookie.Value != state {
		app.logger.PrintInfo("State token mismatch", map[string]string{
			"received_state": state,
			"cookie_state":   cookie.Value,
		})
		app.invalidCredentialsResponse(w, r)
		return
	}

	// Exchange code for token
	token, err := app.googleOauthConfig.Exchange(r.Context(), code)
	if err != nil {
		app.logger.PrintError(err, map[string]string{"message": "Token exchange failed"})
		app.badRequestResponse(w, r, err)
		return
	}

	// Get user info from Google
	googleUser, err := app.getGoogleUserInfo(token.AccessToken)
	if err != nil {
		app.logger.PrintError(err, map[string]string{"message": "Failed to get Google user info"})
		app.serverErrorResponse(w, r, err)
		return
	}

	existingUser, err := app.models.Users.GetByEmail(googleUser.Email)
	if err == nil {
		authToken, err := app.models.Tokens.New(existingUser.ID, 24*time.Hour, data.ScopeAuthentication)
		if err != nil {
			app.logger.PrintError(err, map[string]string{"message": "Failed to generate authentication token"})
			app.serverErrorResponse(w, r, err)
			return
		}

		redirectURL := fmt.Sprintf("%s/auth/callback?token=%s", app.config.frontendURL, authToken.Plaintext)
		app.logger.PrintInfo("Redirecting existing user to frontend", map[string]string{
			"email": existingUser.Email,
			"url":   redirectURL,
		})
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	} else if !errors.Is(err, data.ErrRecordNotFound) {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Find or create user
	user, err := app.models.Users.FindOrCreateFromGoogle(googleUser)
	if err != nil {
		app.logger.PrintError(err, map[string]string{"message": "Failed to find or create user"})
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Permissions.AddForUser(user.ID, "ideas:write")

	fmt.Println("User ID: ", user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}


	// Generate authentication token
	authToken, err := app.models.Tokens.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.logger.PrintError(err, map[string]string{"message": "Failed to generate authentication token"})
		app.serverErrorResponse(w, r, err)
		return
	}

	redirectURL := fmt.Sprintf("%s/auth/callback?token=%s", app.config.frontendURL, authToken.Plaintext)
	app.logger.PrintInfo("Redirecting to frontend", map[string]string{
		"url": redirectURL,
	})
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	expiration := time.Now().Add(365 * 24 * time.Hour)
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	cookie := http.Cookie{
		Name:     "oauthstate",
		Value:    state,
		Expires:  expiration,
		HttpOnly: true,
	}
	http.SetCookie(w, &cookie)
	return state
}

func (app *application) getGoogleUserInfo(accessToken string) (*data.GoogleUser, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var user data.GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}
