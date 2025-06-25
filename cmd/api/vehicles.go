package main

import (
	"errors"
	"net/http"

	"github.com/mayura-andrew/SpotLinkIO-backend/internal/data"
	"github.com/mayura-andrew/SpotLinkIO-backend/internal/validator"
)

// Create a new vehicle for the authenticated user
func (app *application) createVehicleHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		LicensePlate string `json:"license_plate"`
		Make         string `json:"make"`
		Model        string `json:"model"`
		Color        string `json:"color"`
		VehicleType  string `json:"vehicle_type"`
		IsDefault    *bool  `json:"is_default"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Get the authenticated user
	user := app.contextGetUser(r)

	// Create vehicle instance
	vehicle := &data.Vehicle{
		UserID:       user.ID,
		LicensePlate: input.LicensePlate,
		Make:         input.Make,
		Model:        input.Model,
		Color:        input.Color,
		VehicleType:  input.VehicleType,
		IsDefault:    false, // Default to false
	}

	// Set as default if specified
	if input.IsDefault != nil {
		vehicle.IsDefault = *input.IsDefault
	}

	// Validate the vehicle
	v := validator.New()
	if data.ValidateVehicle(v, vehicle); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Insert the vehicle
	err = app.models.Vehicles.Insert(vehicle)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateLicensePlate):
			v.AddError("license_plate", "a vehicle with this license plate already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Return the created vehicle
	err = app.writeJSON(w, http.StatusCreated, envelope{"vehicle": vehicle}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// Get all vehicles for the authenticated user
func (app *application) listVehiclesHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		data.Filters
	}

	v := validator.New()
	qs := r.URL.Query()

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "id")
	input.Filters.SortSafelist = []string{"id", "license_plate", "make", "model", "created_at", "-id", "-license_plate", "-make", "-model", "-created_at"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Get the authenticated user
	user := app.contextGetUser(r)

	// Get vehicles for this user
	vehicles, metadata, err := app.models.Vehicles.GetAllForUser(user.ID, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"vehicles": vehicles, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// Get a specific vehicle by ID
func (app *application) showVehicleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Get the vehicle
	vehicle, err := app.models.Vehicles.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the vehicle belongs to the authenticated user
	user := app.contextGetUser(r)
	if vehicle.UserID != user.ID {
		app.notPermittedResponse(w, r)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"vehicle": vehicle}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// Update a vehicle
func (app *application) updateVehicleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Get the existing vehicle
	vehicle, err := app.models.Vehicles.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the vehicle belongs to the authenticated user
	user := app.contextGetUser(r)
	if vehicle.UserID != user.ID {
		app.notPermittedResponse(w, r)
		return
	}

	var input struct {
		LicensePlate *string `json:"license_plate"`
		Make         *string `json:"make"`
		Model        *string `json:"model"`
		Color        *string `json:"color"`
		VehicleType  *string `json:"vehicle_type"`
		IsDefault    *bool   `json:"is_default"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Update only provided fields
	if input.LicensePlate != nil {
		vehicle.LicensePlate = *input.LicensePlate
	}
	if input.Make != nil {
		vehicle.Make = *input.Make
	}
	if input.Model != nil {
		vehicle.Model = *input.Model
	}
	if input.Color != nil {
		vehicle.Color = *input.Color
	}
	if input.VehicleType != nil {
		vehicle.VehicleType = *input.VehicleType
	}
	if input.IsDefault != nil {
		vehicle.IsDefault = *input.IsDefault
	}

	// Validate the updated vehicle
	v := validator.New()
	if data.ValidateVehicle(v, vehicle); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Update the vehicle
	err = app.models.Vehicles.Update(vehicle)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateLicensePlate):
			v.AddError("license_plate", "a vehicle with this license plate already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"vehicle": vehicle}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// Delete a vehicle
func (app *application) deleteVehicleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Get the vehicle to check ownership
	vehicle, err := app.models.Vehicles.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the vehicle belongs to the authenticated user
	user := app.contextGetUser(r)
	if vehicle.UserID != user.ID {
		app.notPermittedResponse(w, r)
		return
	}

	// Delete the vehicle
	err = app.models.Vehicles.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "vehicle successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// Set a vehicle as default
func (app *application) setDefaultVehicleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Get the vehicle to check ownership
	vehicle, err := app.models.Vehicles.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the vehicle belongs to the authenticated user
	user := app.contextGetUser(r)
	if vehicle.UserID != user.ID {
		app.notPermittedResponse(w, r)
		return
	}

	// Set as default
	err = app.models.Vehicles.SetAsDefault(user.ID, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Get the updated vehicle
	vehicle, err = app.models.Vehicles.Get(id)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{
		"vehicle": vehicle,
		"message": "vehicle set as default successfully",
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
