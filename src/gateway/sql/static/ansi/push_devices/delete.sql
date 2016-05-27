DELETE FROM push_devices
WHERE push_devices.id = ?
  AND push_devices.push_channel_id IN
    (SELECT push_channels.id
      FROM push_channels
      WHERE push_channels.id = ?
        AND push_channels.account_id = ?);
