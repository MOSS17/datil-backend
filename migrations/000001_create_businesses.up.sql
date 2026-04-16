CREATE TABLE businesses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    location VARCHAR(255),
    description TEXT,
    logo VARCHAR(255),
    url VARCHAR(255) NOT NULL UNIQUE,
    beneficiary_clabe VARCHAR(255),
    bank_name VARCHAR(255),
    beneficiary_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((beneficiary_clabe IS NULL AND bank_name IS NULL AND beneficiary_name IS NULL)
        OR (beneficiary_clabe IS NOT NULL AND bank_name IS NOT NULL AND beneficiary_name IS NOT NULL))
);
