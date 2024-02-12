CREATE TABLE IF NOT EXISTS "users" (
    "id" SERIAL PRIMARY KEY,
    "user_id" varchar(255) NOT NULL,
    "email" varchar(255) DEFAULT NULL,
    "password" varchar(255) DEFAULT NULL,
    "created_at" timestamp DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp DEFAULT CURRENT_TIMESTAMP
);