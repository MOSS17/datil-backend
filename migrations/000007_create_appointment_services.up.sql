CREATE TABLE appointment_services (
    appointment_id UUID NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id),
    price DECIMAL(10, 2) NOT NULL,
    duration INT NOT NULL,
    PRIMARY KEY (appointment_id, service_id)
);
