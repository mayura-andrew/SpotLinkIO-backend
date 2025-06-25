package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Permissions     PermissionModel
	Users           UserModal
	Tokens          TokenModel
	Vehicles        VehicleModel
	QRCodes         QRCodeModel
	ParkingLots     ParkingLotModel
	ParkingSpots    ParkingSpotModel
	Reservations    ReservationModel
	Payments        PaymentModel
	ParkingSessions ParkingSessionModel
	Notifications   NotificationModel
	Reviews         ReviewModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Permissions: PermissionModel{DB: db},
		Users:       UserModal{DB: db},
		Tokens:      TokenModel{DB: db},
		Vehicles:    VehicleModel{DB: db},
		QRCodes:     QRCodeModel{DB: db},
		ParkingLots:     ParkingLotModel{DB: db},
		ParkingSpots:    ParkingSpotModel{DB: db},
		Reservations:    ReservationModel{DB: db},
		Payments:        PaymentModel{DB: db},
		ParkingSessions: ParkingSessionModel{DB: db},
		Notifications:   NotificationModel{DB: db},
		Reviews:         ReviewModel{DB: db},
	}
}
