INSERT INTO proxy_endpoints (
  api_id,
  name, description,
  endpoint_group_id,
  environment_id,
  active, cors_enabled, routes
)
VALUES (
  (SELECT id FROM apis WHERE id = ? AND account_id = ?),
  ?, ?,
  (SELECT id FROM endpoint_groups WHERE id = ? AND api_id = ?),
  (SELECT id FROM environments WHERE id = ? AND api_id = ?),
  ?, ?, ?
)
