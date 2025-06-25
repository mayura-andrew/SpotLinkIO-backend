CREATE TABLE IF NOT EXISTS parking_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id UUID REFERENCES reservations ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES vehicles ON DELETE CASCADE,
    parking_spot_id UUID NOT NULL REFERENCES parking_spots ON DELETE CASCADE,
    check_in_time TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    check_out_time TIMESTAMP(0) WITH TIME ZONE,
    status TEXT NOT NULL DEFAULT 'active',
    total_duration INTEGER,
    total_amount DECIMAL(10, 2),
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1
);
