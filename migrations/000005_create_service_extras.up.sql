CREATE TABLE service_extras (
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    extra_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    PRIMARY KEY (service_id, extra_id)
);
