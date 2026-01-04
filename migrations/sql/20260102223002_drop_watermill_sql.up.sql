-- Drop Watermill SQL tables (migrating to NATS-based pubsub)
DROP TABLE IF EXISTS watermill_offsets;
DROP TABLE IF EXISTS watermill_messages;
