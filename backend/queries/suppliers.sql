-- name: SearchSuppliers :many
SELECT id, name, credit_code, business_scope, registered_capital, paid_capital,
       employee_count, establish_date, province, city, is_manufacturer, risk_level,
       risk_tags, certifications, main_products, capacity_description, created_at, updated_at
FROM suppliers
WHERE (sqlc.narg('business_keyword')::text IS NULL OR business_scope ILIKE '%' || sqlc.narg('business_keyword') || '%')
  AND (sqlc.narg('province')::text IS NULL OR province = sqlc.narg('province'))
  AND (sqlc.narg('city')::text IS NULL OR city = sqlc.narg('city'))
  AND (sqlc.narg('min_capital')::bigint IS NULL OR registered_capital >= sqlc.narg('min_capital'))
  AND (sqlc.narg('min_employees')::int IS NULL OR employee_count >= sqlc.narg('min_employees'))
ORDER BY updated_at DESC;

-- name: GetSupplier :one
SELECT * FROM suppliers WHERE id = $1;
