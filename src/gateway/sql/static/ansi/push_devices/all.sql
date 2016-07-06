SELECT
  push_devices.id as id,
  push_devices.remote_endpoint_id as remote_endpoint_id,
  push_devices.name as name,
  push_devices.type as type,
  push_devices.token as token,
  push_devices.data as data
FROM push_devices, remote_endpoints, apis, accounts
WHERE push_devices.remote_endpoint_id = remote_endpoints.id
  AND remote_endpoints.api_id = apis.id
  AND apis.account_id = accounts.id
  AND accounts.id = ?
ORDER BY push_devices.id ASC;
