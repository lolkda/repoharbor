-- +goose Up
ALTER TABLE repositories ADD COLUMN cron_expression TEXT NOT NULL DEFAULT '0 * * * *';

UPDATE repositories
SET cron_expression = CASE
  WHEN interval_minutes = 15 THEN '*/15 * * * *'
  WHEN interval_minutes = 30 THEN '*/30 * * * *'
  WHEN interval_minutes = 60 THEN '0 * * * *'
  WHEN interval_minutes = 360 THEN '0 */6 * * *'
  WHEN interval_minutes = 720 THEN '0 */12 * * *'
  WHEN interval_minutes = 1440 THEN '0 0 * * *'
  WHEN interval_minutes > 0 THEN '*/' || interval_minutes || ' * * * *'
  ELSE '0 * * * *'
END;

ALTER TABLE repositories DROP COLUMN interval_minutes;

-- +goose Down
ALTER TABLE repositories ADD COLUMN interval_minutes INTEGER NOT NULL DEFAULT 60;

UPDATE repositories
SET interval_minutes = CASE
  WHEN cron_expression = '*/15 * * * *' THEN 15
  WHEN cron_expression = '*/30 * * * *' THEN 30
  WHEN cron_expression = '0 * * * *' THEN 60
  WHEN cron_expression = '0 */6 * * *' THEN 360
  WHEN cron_expression = '0 */12 * * *' THEN 720
  WHEN cron_expression = '0 0 * * *' THEN 1440
  ELSE 60
END;

ALTER TABLE repositories DROP COLUMN cron_expression;
