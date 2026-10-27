CREATE TABLE IF NOT EXISTS gift_cards (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    amount NUMERIC NOT NULL,
    expires_at TIMESTAMPTZ,
    redeemed_by_user_id BIGINT,
    redeemed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_gift_cards_code ON gift_cards (code);
CREATE INDEX IF NOT EXISTS idx_gift_cards_redeemed_by_user_id ON gift_cards (redeemed_by_user_id);
