package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/password-reset", app.updateUserPasswordHandler)

	router.HandlerFunc(http.MethodPost, "/v1/auth/tokens/authentication", app.createAuthenticationTokenHandler)
	router.HandlerFunc(http.MethodPost, "/v1/auth/tokens/password-reset-request", app.createPasswordResetTokenHandler)

	router.HandlerFunc(http.MethodGet, "/v1/auth/google/login", app.googleLoginHandler)
	router.HandlerFunc(http.MethodGet, "/v1/auth/google/callback", app.googleCallbackHandler)


	router.HandlerFunc(http.MethodGet, "/v1/files/:type/:id", app.serveFilesHandler)
	router.HandlerFunc(http.MethodGet, "/v1/avatars/:id", app.serveAvatarHandler) // Direct avatar access
	router.HandlerFunc(http.MethodGet, "/v1/pdfs/:id", app.servePDFHandler)       // Direct PDF access

	//router.HandlerFunc(http.MethodGet, "/v1/profiles/:username", app.requirePermission("ideas:read", app.getProfileByUsernameHandler))

	return app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router))))

}
