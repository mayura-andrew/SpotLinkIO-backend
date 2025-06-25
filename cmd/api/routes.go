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

	router.HandlerFunc(http.MethodGet, "/v1/users/profile", app.requireActivatedUser(app.getUserProfileHandler))
	router.HandlerFunc(http.MethodPost, "/v1/users/complete-profile", app.requireActivatedUser(app.completeProfileHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/users/profile", app.requireActivatedUser(app.updateUserProfileHandler))

	// Vehicle routes (require authentication)
	router.HandlerFunc(http.MethodPost, "/v1/vehicles", app.requireActivatedUser(app.createVehicleHandler))
	router.HandlerFunc(http.MethodGet, "/v1/vehicles", app.requireActivatedUser(app.listVehiclesHandler))
	router.HandlerFunc(http.MethodGet, "/v1/vehicles/:id", app.requireActivatedUser(app.showVehicleHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/vehicles/:id", app.requireActivatedUser(app.updateVehicleHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/vehicles/:id", app.requireActivatedUser(app.deleteVehicleHandler))
	router.HandlerFunc(http.MethodPut, "/v1/vehicles/:id/set-default", app.requireActivatedUser(app.setDefaultVehicleHandler))

	//router.HandlerFunc(http.MethodGet, "/v1/profiles/:username", app.requirePermission("ideas:read", app.getProfileByUsernameHandler))

	router.HandlerFunc(http.MethodPost, "/v1/qr-codes/generate", app.requireActivatedUser(app.generateQRCodeHandler))
	router.HandlerFunc(http.MethodPost, "/v1/qr-codes/verify", app.verifyQRCodeHandler)
	router.HandlerFunc(http.MethodGet, "/v1/qr-codes", app.requireActivatedUser(app.getUserQRCodesHandler))
	router.HandlerFunc(http.MethodGet, "/v1/qr-images/:filename", app.serveQRImageHandler)
	return app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router))))

}
