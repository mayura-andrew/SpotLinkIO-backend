package main

import (
    "errors"
    "net/http"
    "os"
    "path/filepath"

    "github.com/google/uuid"
    "github.com/julienschmidt/httprouter"
    "github.com/mayura-andrew/SpotLinkIO-backend/internal/data"
    "github.com/mayura-andrew/SpotLinkIO-backend/internal/qrcode"
    "github.com/mayura-andrew/SpotLinkIO-backend/internal/validator"
)

func (app *application) generateQRCodeHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        VehicleID    string `json:"vehicle_id"`
        ExpiryHours  *int   `json:"expiry_hours"`
        Purpose      string `json:"purpose"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    // Validate input
    v := validator.New()
    v.Check(input.VehicleID != "", "vehicle_id", "must be provided")
    v.Check(validator.PermittedValue(input.Purpose, "parking", "identification", "emergency"), "purpose", "must be a valid purpose")

    vehicleID, err := uuid.Parse(input.VehicleID)
    if err != nil {
        v.AddError("vehicle_id", "must be a valid UUID")
    }

    // Set default expiry to 24 hours if not provided
    expiryHours := 24
    if input.ExpiryHours != nil {
        expiryHours = *input.ExpiryHours
        v.Check(expiryHours > 0 && expiryHours <= 168, "expiry_hours", "must be between 1 and 168 hours (7 days)")
    }

    if !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Get authenticated user
    user := app.contextGetUser(r)

    // Create QR code service
    qrService := qrcode.NewService(app.models, app.config.qr.storageDir)

    // Generate QR code
    qrResponse, err := qrService.GenerateQRCode(user.ID, vehicleID, expiryHours, input.Purpose)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.notFoundResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    err = app.writeJSON(w, http.StatusCreated, envelope{
        "qr_code":    qrResponse.QRCode,
        "qr_data":    qrResponse.QRData,
        "image_url":  qrResponse.ImageURL,
        "verify_url": qrResponse.VerifyURL,
        "message":    "QR code generated successfully",
    }, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) verifyQRCodeHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Code string `json:"code"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    v := validator.New()
    v.Check(input.Code != "", "code", "must be provided")

    if !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Create QR code service
    qrService := qrcode.NewService(app.models, app.config.qr.storageDir)

    // Verify QR code
    qrData, err := qrService.VerifyQRCode(input.Code)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.errorResponse(w, r, http.StatusNotFound, "QR code not found or expired")
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{
        "qr_data": qrData,
        "message": "QR code verified successfully",
    }, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) getUserQRCodesHandler(w http.ResponseWriter, r *http.Request) {
    user := app.contextGetUser(r)

    qrCodes, err := app.models.QRCodes.GetActiveForUser(user.ID)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{
        "qr_codes": qrCodes,
        "count":    len(qrCodes),
    }, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) serveQRImageHandler(w http.ResponseWriter, r *http.Request) {
    params := httprouter.ParamsFromContext(r.Context())
    filename := params.ByName("filename")

    // Validate filename to prevent directory traversal
    if filename == "" || filepath.Base(filename) != filename {
        app.notFoundResponse(w, r)
        return
    }

    imagePath := filepath.Join(app.config.qr.storageDir, filename)

    // Check if file exists
    if _, err := os.Stat(imagePath); os.IsNotExist(err) {
        app.notFoundResponse(w, r)
        return
    }

    w.Header().Set("Content-Type", "image/png")
    w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

    http.ServeFile(w, r, imagePath)
}