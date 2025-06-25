package qrcode

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/google/uuid"
    "github.com/mayura-andrew/SpotLinkIO-backend/internal/data"
    "github.com/skip2/go-qrcode"
)

type Service struct {
    models     data.Models
    storageDir string
}

func NewService(models data.Models, storageDir string) *Service {
    // Ensure storage directory exists
    os.MkdirAll(storageDir, 0755)
    
    return &Service{
        models:     models,
        storageDir: storageDir,
    }
}

func (s *Service) GenerateQRCode(userID, vehicleID uuid.UUID, expiryHours int, purpose string) (*QRCodeResponse, error) {
    // Get user data
    user, err := s.models.Users.Get(userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }

    // Get vehicle data
    vehicle, err := s.models.Vehicles.Get(vehicleID)
    if err != nil {
        return nil, fmt.Errorf("failed to get vehicle: %w", err)
    }

    // Verify vehicle belongs to user
    if vehicle.UserID != userID {
        return nil, fmt.Errorf("vehicle does not belong to user")
    }

    // Generate unique code
    code, err := s.generateUniqueCode()
    if err != nil {
        return nil, fmt.Errorf("failed to generate code: %w", err)
    }

    // Create QR data
    expiresAt := time.Now().Add(time.Duration(expiryHours) * time.Hour)
    qrData := data.QRCodeData{
        UserProfile: data.UserProfile{
            ID:           user.ID,
            UserName:     user.UserName,
            FirstName:    user.FirstName,
            LastName:     user.LastName,
            MobileNumber: user.MobileNumber,
            Email:        user.Email,
        },
        Vehicle: data.VehicleData{
            ID:           vehicle.ID,
            LicensePlate: vehicle.LicensePlate,
            Make:         vehicle.Make,
            Model:        vehicle.Model,
            Color:        vehicle.Color,
            VehicleType:  vehicle.VehicleType,
        },
        QRInfo: data.QRCodeInfo{
            Code:        code,
            GeneratedAt: time.Now(),
            ExpiresAt:   expiresAt,
            Purpose:     purpose,
        },
    }

    // Marshal to JSON
    dataJSON, err := json.Marshal(qrData)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal QR data: %w", err)
    }

    // Create QR code record
    qrCodeRecord := &data.QRCode{
        UserID:    userID,
        VehicleID: vehicleID,
        Code:      code,
        Data:      string(dataJSON),
        ExpiresAt: expiresAt,
        IsActive:  true,
    }

    // Deactivate previous QR codes for this user (optional - based on business logic)
    err = s.models.QRCodes.DeactivateAllForUser(userID)
    if err != nil {
        return nil, fmt.Errorf("failed to deactivate previous QR codes: %w", err)
    }

    // Save to database
    err = s.models.QRCodes.Insert(qrCodeRecord)
    if err != nil {
        return nil, fmt.Errorf("failed to save QR code: %w", err)
    }

    // Generate QR code image
    imageFilename := fmt.Sprintf("qr_%s.png", code)
    imagePath := filepath.Join(s.storageDir, imageFilename)

    // Create QR verification URL (this would be your frontend URL)
    verificationURL := fmt.Sprintf("https://spotlinkio.com/verify?code=%s", code)

    err = qrcode.WriteFile(verificationURL, qrcode.Medium, 256, imagePath)
    if err != nil {
        return nil, fmt.Errorf("failed to generate QR image: %w", err)
    }

    return &QRCodeResponse{
        QRCode:      qrCodeRecord,
        QRData:      qrData,
        ImagePath:   imagePath,
        ImageURL:    fmt.Sprintf("/v1/qr-images/%s", imageFilename),
        VerifyURL:   verificationURL,
    }, nil
}

func (s *Service) VerifyQRCode(code string) (*data.QRCodeData, error) {
    qrCode, err := s.models.QRCodes.GetByCode(code)
    if err != nil {
        return nil, err
    }

    var qrData data.QRCodeData
    err = json.Unmarshal([]byte(qrCode.Data), &qrData)
    if err != nil {
        return nil, fmt.Errorf("failed to parse QR data: %w", err)
    }

    return &qrData, nil
}

func (s *Service) generateUniqueCode() (string, error) {
    bytes := make([]byte, 32)
    _, err := rand.Read(bytes)
    if err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes)[:32], nil
}

type QRCodeResponse struct {
    QRCode    *data.QRCode     `json:"qr_code"`
    QRData    data.QRCodeData  `json:"qr_data"`
    ImagePath string           `json:"-"`
    ImageURL  string           `json:"image_url"`
    VerifyURL string           `json:"verify_url"`
}