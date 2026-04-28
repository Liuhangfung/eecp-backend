CREATE TABLE IF NOT EXISTS machines (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS bookings (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    machine_id INTEGER NOT NULL REFERENCES machines(id),
    telegram_user_id BIGINT NOT NULL,
    telegram_username TEXT NOT NULL DEFAULT '',
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'confirmed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_no_double_book
    ON bookings (machine_id, start_time)
    WHERE status = 'confirmed';

CREATE INDEX IF NOT EXISTS idx_bookings_user
    ON bookings (telegram_user_id, status);

CREATE INDEX IF NOT EXISTS idx_bookings_time
    ON bookings (start_time, end_time)
    WHERE status = 'confirmed';

CREATE TABLE IF NOT EXISTS admins (
    telegram_user_id BIGINT PRIMARY KEY
);

INSERT INTO machines (name) VALUES
    ('EECP Machine #1'),
    ('EECP Machine #2'),
    ('EECP Machine #3'),
    ('EECP Machine #4'),
    ('EECP Machine #5')
ON CONFLICT DO NOTHING;
