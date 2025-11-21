ALTER TABLE aws_connections
  ADD COLUMN region VARCHAR(32) NOT NULL DEFAULT 'ap-northeast-1';
