package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/mayura-andrew/SpotLinkIO-backend/internal/data"
	"github.com/mayura-andrew/SpotLinkIO-backend/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserName string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	_, err = app.models.Users.GetByEmail(input.Email)
	if err == nil {
		app.failedValidationResponse(w, r, map[string]string{"email": "a user with this email address already exists"})
		return
	} else if !errors.Is(err, data.ErrRecordNotFound) {
		app.serverErrorResponse(w, r, err)
		return
	}

	user := &data.User{
		UserName:               input.UserName,
		Email:                  input.Email,
		Role:                   "normal",
		AuthType:               "normal",
		Activated:              false,
		HasCompletedOnboarding: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Permissions.AddForUser(user.ID, "ideas:read")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	token, err := app.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {

		emailData := map[string]any{
			"activationToken": token.Plaintext,
			"userName":        user.UserName,
			"frontendURL":     app.config.frontendURL,
		}
		err = app.mailer.Send(user.Email, "user_welcome", emailData)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlainText string `json:"token"`
	}

	err := app.readJSON(w, r, &input)

	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateTokenPlaintext(v, input.TokenPlainText); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetForToken(data.ScopeActivation, input.TokenPlainText)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	user.Activated = true

	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Permissions.AddForUser(user.ID, "ideas:write")

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateUserPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password       string `json:"password"`
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	data.ValidatePasswordPlaintext(v, input.Password)
	data.ValidateTokenPlaintext(v, input.TokenPlaintext)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.Users.GetForToken(data.ScopePasswordReset, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopePasswordReset, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	env := envelope{"message": "your password was successfully reset"}

	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) completeProfileHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        FirstName    string  `json:"first_name"`
        LastName     string  `json:"last_name"`
        MobileNumber *string `json:"mobile_number"`
        AvatarURL    *string `json:"avatar_url"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    // Get the authenticated user from context
    user := app.contextGetUser(r)

    // Check if user is activated
    if !user.Activated {
        app.errorResponse(w, r, http.StatusForbidden, "user account must be activated first")
        return
    }

    // Check if profile is already completed
    if user.HasCompletedOnboarding {
        app.errorResponse(w, r, http.StatusBadRequest, "profile has already been completed")
        return
    }

    // Update user profile fields
    user.FirstName = &input.FirstName
    user.LastName = &input.LastName
    user.MobileNumber = input.MobileNumber
    user.AvatarURL = input.AvatarURL
    user.HasCompletedOnboarding = true

    // Validate the profile data
    v := validator.New()
    if data.ValidateProfile(v, user); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Update the user in the database
    err = app.models.Users.UpdateProfile(user)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrEditConflict):
            app.editConflictResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // Return the updated user
    err = app.writeJSON(w, http.StatusOK, envelope{
        "user":    user,
        "message": "profile completed successfully",
    }, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) getUserProfileHandler(w http.ResponseWriter, r *http.Request) {
    user := app.contextGetUser(r)

    err := app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) updateUserProfileHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        FirstName    *string `json:"first_name"`
        LastName     *string `json:"last_name"`
        MobileNumber *string `json:"mobile_number"`
        AvatarURL    *string `json:"avatar_url"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    user := app.contextGetUser(r)

    // Update only provided fields
    if input.FirstName != nil {
        user.FirstName = input.FirstName
    }
    if input.LastName != nil {
        user.LastName = input.LastName
    }
    if input.MobileNumber != nil {
        user.MobileNumber = input.MobileNumber
    }
    if input.AvatarURL != nil {
        user.AvatarURL = input.AvatarURL
    }

    // Validate the profile data
    v := validator.New()
    if data.ValidateProfile(v, user); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Update the user in the database
    err = app.models.Users.UpdateProfile(user)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrEditConflict):
            app.editConflictResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}