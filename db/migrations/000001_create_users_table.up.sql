BEGIN;
DROP TYPE IF EXISTS order_state;
create type order_state as enum ('REGISTERED', 'INVALID', 'PROCESSING', 'PROCESSED', 'NEW');
CREATE TABLE IF NOT EXISTS users (
                                     id SERIAL PRIMARY KEY,
                                     login VARCHAR(255) NOT NULL UNIQUE,
                                     password VARCHAR(255) NOT NULL
);
CREATE TABLE IF NOT EXISTS balance (
    id SERIAL PRIMARY KEY,
    user_id int references users(id),
    accrual int default 0 ,
    order_id varchar(255) NOT NULL UNIQUE,
    order_status order_state DEFAULT 'NEW',
    created_at timestamp with time zone default now()
);
COMMIT;

