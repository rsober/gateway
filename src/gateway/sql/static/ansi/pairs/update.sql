UPDATE proxy_endpoint_test_pairs
SET
  key = ?,
  value = ?
WHERE id = ? AND test_id = ?;
