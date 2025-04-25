WITH
    events AS (
        SELECT
            *,
            (
                SELECT dim.values[OFFSET(0)]
                FROM UNNEST(be.bot.dimensions) AS dim
                WHERE dim.key = 'label-pool' LIMIT 1
    ) AS pool,
FROM swarming.bot_events AS be
WHERE (event_time >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 14 DAY))
    ),
    last_bot_id_events AS (
SELECT
    bot.bot_id AS bot_id,
    MAX(event_time) AS last_event_ts
FROM events
GROUP BY bot.bot_id
    ),
    bot_last_event AS (
SELECT
    bot.dimensions AS dimensions
FROM events
    JOIN last_bot_id_events AS last_event
ON (events.bot.bot_id = last_event.bot_id) AND
    (events.event_time = last_event.last_event_ts)
WHERE pool = '{}'
    )
SELECT
    (
        SELECT dim.values[OFFSET(0)]
        FROM UNNEST(dimensions) AS dim
        WHERE dim.key = 'hwid' LIMIT 1
    ) AS hwid
FROM bot_last_event AS bot
WHERE dimensions IS NOT NULL
  AND (SELECT dim.values[OFFSET(0)] FROM UNNEST(dimensions) AS dim WHERE dim.key = 'hwid' LIMIT 1) IS NOT NULL