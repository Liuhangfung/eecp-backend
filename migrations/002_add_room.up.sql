ALTER TABLE machines ADD COLUMN IF NOT EXISTS room TEXT NOT NULL DEFAULT 'common';
UPDATE machines SET room = 'vip' WHERE id IN (1, 2);
UPDATE machines SET room = 'common' WHERE id IN (3, 4, 5);
