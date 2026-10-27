CREATE TABLE suppliers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  credit_code CHAR(18) NOT NULL UNIQUE,
  business_scope TEXT NOT NULL,
  registered_capital BIGINT NOT NULL CHECK (registered_capital >= 0),
  paid_capital BIGINT NOT NULL CHECK (paid_capital >= 0),
  employee_count INTEGER NOT NULL CHECK (employee_count >= 0),
  establish_date DATE NOT NULL,
  province TEXT NOT NULL,
  city TEXT NOT NULL,
  is_manufacturer BOOLEAN NOT NULL DEFAULT false,
  risk_level TEXT NOT NULL CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
  risk_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  certifications JSONB NOT NULL DEFAULT '[]'::jsonb,
  main_products JSONB NOT NULL DEFAULT '[]'::jsonb,
  capacity_description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX suppliers_location_idx ON suppliers (province, city);
CREATE INDEX suppliers_capital_idx ON suppliers (registered_capital);
CREATE INDEX suppliers_risk_idx ON suppliers (risk_level);
