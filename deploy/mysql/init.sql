CREATE TABLE IF NOT EXISTS users (
  id CHAR(36) PRIMARY KEY,
  phone VARCHAR(32) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  name VARCHAR(80) NOT NULL,
  role VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS service_categories (
  id CHAR(36) PRIMARY KEY,
  name VARCHAR(80) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS services (
  id CHAR(36) PRIMARY KEY,
  category_id CHAR(36) NULL,
  name VARCHAR(120) NOT NULL,
  skill VARCHAR(80) NOT NULL,
  area VARCHAR(80) NOT NULL,
  slot_capacity INT NOT NULL DEFAULT 1,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_services_category_id (category_id)
);

CREATE TABLE IF NOT EXISTS addresses (
  id CHAR(36) PRIMARY KEY,
  user_id CHAR(36) NOT NULL,
  contact_name VARCHAR(80) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  detail VARCHAR(255) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_addresses_user_id (user_id)
);

CREATE TABLE IF NOT EXISTS time_slots (
  id CHAR(36) PRIMARY KEY,
  service_id CHAR(36) NOT NULL,
  service_date DATE NOT NULL,
  starts_at DATETIME NOT NULL,
  capacity INT NOT NULL,
  used_count INT NOT NULL DEFAULT 0,
  UNIQUE KEY uk_time_slots_service_date_id (service_id, service_date, id)
);

CREATE TABLE IF NOT EXISTS technicians (
  id CHAR(36) PRIMARY KEY,
  user_id CHAR(36) NOT NULL,
  name VARCHAR(80) NOT NULL,
  load_count INT NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
  id CHAR(36) PRIMARY KEY,
  user_id CHAR(36) NOT NULL,
  service_id CHAR(36) NOT NULL,
  address_id CHAR(36) NOT NULL,
  service_date DATE NOT NULL,
  slot_id CHAR(36) NOT NULL,
  technician_id CHAR(36) NULL,
  remark VARCHAR(500) NULL,
  state VARCHAR(64) NOT NULL,
  idempotency_key VARCHAR(128) NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_orders_user_id_created_at (user_id, created_at),
  INDEX idx_orders_state_date (state, service_date)
);

CREATE TABLE IF NOT EXISTS order_events (
  id CHAR(36) PRIMARY KEY,
  order_id CHAR(36) NOT NULL,
  from_state VARCHAR(64) NULL,
  to_state VARCHAR(64) NOT NULL,
  actor_id CHAR(36) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_order_events_order_id (order_id)
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id CHAR(36) PRIMARY KEY,
  actor_id CHAR(36) NOT NULL,
  action VARCHAR(64) NOT NULL,
  resource VARCHAR(128) NOT NULL,
  result VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_audit_logs_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS reviews (
  id CHAR(36) PRIMARY KEY,
  order_id CHAR(36) NOT NULL UNIQUE,
  user_id CHAR(36) NOT NULL,
  rating TINYINT NOT NULL,
  content VARCHAR(1000) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS work_proofs (
  id CHAR(36) PRIMARY KEY,
  order_id CHAR(36) NOT NULL,
  kind VARCHAR(16) NOT NULL,
  filename VARCHAR(255) NOT NULL,
  note VARCHAR(500) NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_work_proofs_order_id (order_id)
);
