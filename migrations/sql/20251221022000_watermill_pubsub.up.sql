CREATE TABLE IF NOT EXISTS watermill_messages (
    "offset" BIGSERIAL,
    "uuid" VARCHAR(36) NOT NULL,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "payload" JSONB DEFAULT NULL,
    "metadata" JSONB DEFAULT NULL,
    "topic" VARCHAR(255) NOT NULL,
    "transaction_id" XID8 NOT NULL,
    PRIMARY KEY (topic, "transaction_id", "offset")
);

CREATE TABLE IF NOT EXISTS watermill_offsets (
    consumer_group VARCHAR(255) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    offset_acked BIGINT NOT NULL,
    last_processed_transaction_id XID8 NOT NULL,
    PRIMARY KEY (consumer_group, topic)
);
