-- +goose Up
-- Contoh skema awal. Ganti/hapus sesuai kebutuhanmu (dan jalankan `make migrate-create`).
CREATE TABLE users (
    id         uuid PRIMARY KEY,
    email      text        NOT NULL UNIQUE,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
