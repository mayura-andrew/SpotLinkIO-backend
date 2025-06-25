CREATE TABLE IF NOT EXISTS reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES vehicles ON DELETE CASCADE,
    parking_lot_id UUID NOT NULL REFERENCES parking_lots ON DELETE CASCADE,
    parking_spot_id UUID REFERENCES parking_spots ON DELETE SET NULL,
    start_time TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    actual_start_time TIMESTAMP(0) WITH TIME ZONE,
    actual_end_time TIMESTAMP(0) WITH TIME ZONE,
    status TEXT NOT NULL DEFAULT 'pending',
    total_amount DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1
);