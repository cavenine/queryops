package pubsub

import (
	stdsql "database/sql"
	"encoding/json"
	"fmt"
	"strings"

	wsql "github.com/ThreeDotsLabs/watermill-sql/v4/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
)

const (
	messagesTableName = "watermill_messages"
	offsetsTableName  = "watermill_offsets"
)

// PostgreSQLSchema stores Watermill messages in a shared table.
//
// Watermill's default PostgreSQL schema creates tables per topic, which is not
// compatible with dynamic topics like per-host subscriptions.
type PostgreSQLSchema struct {
	// SubscribeBatchSize is the number of messages to query at once.
	// Default is 100.
	SubscribeBatchSize int
}

func (s PostgreSQLSchema) batchSize() int {
	if s.SubscribeBatchSize == 0 {
		return 100
	}
	return s.SubscribeBatchSize
}

func (s PostgreSQLSchema) SchemaInitializingQueries(_ wsql.SchemaInitializingQueriesParams) ([]wsql.Query, error) {
	createMessagesTable := `
		CREATE TABLE IF NOT EXISTS ` + messagesTableName + ` (
			"offset" BIGSERIAL,
			"uuid" VARCHAR(36) NOT NULL,
			"created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			"payload" JSONB DEFAULT NULL,
			"metadata" JSONB DEFAULT NULL,
			"topic" VARCHAR(255) NOT NULL,
			"transaction_id" XID8 NOT NULL,
			PRIMARY KEY (topic, "transaction_id", "offset")
		);
	`
	return []wsql.Query{{Query: createMessagesTable}}, nil
}

func (s PostgreSQLSchema) InsertQuery(params wsql.InsertQueryParams) (wsql.Query, error) {
	insertQuery := fmt.Sprintf(
		`INSERT INTO %s (uuid, payload, metadata, topic, transaction_id) VALUES %s`,
		messagesTableName,
		insertMarkers(len(params.Msgs)),
	)

	args, err := insertArgs(params.Topic, params.Msgs)
	if err != nil {
		return wsql.Query{}, err
	}

	return wsql.Query{Query: insertQuery, Args: args}, nil
}

func insertMarkers(count int) string {
	var b strings.Builder
	index := 1
	for i := 0; i < count; i++ {
		b.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,pg_current_xact_id()),", index, index+1, index+2, index+3))
		index += 4
	}
	return strings.TrimRight(b.String(), ",")
}

func insertArgs(topic string, msgs message.Messages) ([]any, error) {
	args := make([]any, 0, len(msgs)*4)
	for _, msg := range msgs {
		metadata, err := json.Marshal(msg.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshalling metadata for message %s: %w", msg.UUID, err)
		}
		args = append(args, msg.UUID, []byte(msg.Payload), metadata, topic)
	}
	return args, nil
}

func (s PostgreSQLSchema) SelectQuery(params wsql.SelectQueryParams) (wsql.Query, error) {
	nextOffsetQuery, err := params.OffsetsAdapter.NextOffsetQuery(wsql.NextOffsetQueryParams{
		Topic:         params.Topic,
		ConsumerGroup: params.ConsumerGroup,
	})
	if err != nil {
		return wsql.Query{}, err
	}

	selectQuery := `
	SELECT * FROM (
		WITH last_processed AS (
			` + nextOffsetQuery.Query + `
		)

		SELECT "offset", transaction_id, uuid, payload, metadata
		FROM ` + messagesTableName + `
		WHERE
			topic = $2
			AND (
				(
					transaction_id = (SELECT last_processed_transaction_id FROM last_processed)
					AND "offset" > (SELECT offset_acked FROM last_processed)
				)
				OR (transaction_id > (SELECT last_processed_transaction_id FROM last_processed))
			)
			AND transaction_id < pg_snapshot_xmin(pg_current_snapshot())
	) AS messages
	ORDER BY
		transaction_id ASC,
		"offset" ASC
	LIMIT ` + fmt.Sprintf("%d", s.batchSize())

	return wsql.Query{Query: selectQuery, Args: nextOffsetQuery.Args}, nil
}

func (s PostgreSQLSchema) UnmarshalMessage(params wsql.UnmarshalMessageParams) (wsql.Row, error) {
	r := wsql.Row{}
	var transactionID wsql.XID8

	if err := params.Row.Scan(&r.Offset, &transactionID, &r.UUID, &r.Payload, &r.Metadata); err != nil {
		return wsql.Row{}, fmt.Errorf("scanning message row: %w", err)
	}

	msg := message.NewMessage(string(r.UUID), r.Payload)
	if r.Metadata != nil {
		if err := json.Unmarshal(r.Metadata, &msg.Metadata); err != nil {
			return wsql.Row{}, fmt.Errorf("unmarshalling metadata: %w", err)
		}
	}

	r.Msg = msg
	r.ExtraData = map[string]any{"transaction_id": transactionID}
	return r, nil
}

func (PostgreSQLSchema) SubscribeIsolationLevel() stdsql.IsolationLevel {
	return stdsql.LevelRepeatableRead
}

// PostgreSQLOffsetsAdapter stores offsets in a shared table.
type PostgreSQLOffsetsAdapter struct{}

func (PostgreSQLOffsetsAdapter) SchemaInitializingQueries(_ wsql.OffsetsSchemaInitializingQueriesParams) ([]wsql.Query, error) {
	createOffsetsTable := `
		CREATE TABLE IF NOT EXISTS ` + offsetsTableName + ` (
			consumer_group VARCHAR(255) NOT NULL,
			topic VARCHAR(255) NOT NULL,
			offset_acked BIGINT NOT NULL,
			last_processed_transaction_id XID8 NOT NULL,
			PRIMARY KEY (consumer_group, topic)
		);
	`
	return []wsql.Query{{Query: createOffsetsTable}}, nil
}

func (PostgreSQLOffsetsAdapter) NextOffsetQuery(params wsql.NextOffsetQueryParams) (wsql.Query, error) {
	return wsql.Query{
		Query: `
			SELECT
				offset_acked,
				last_processed_transaction_id
			FROM ` + offsetsTableName + `
			WHERE consumer_group = $1 AND topic = $2
			FOR UPDATE
		`,
		Args: []any{params.ConsumerGroup, params.Topic},
	}, nil
}

func (PostgreSQLOffsetsAdapter) AckMessageQuery(params wsql.AckMessageQueryParams) (wsql.Query, error) {
	ackQuery := `
		INSERT INTO ` + offsetsTableName + ` (consumer_group, topic, offset_acked, last_processed_transaction_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (consumer_group, topic)
		DO UPDATE SET
			offset_acked = excluded.offset_acked,
			last_processed_transaction_id = excluded.last_processed_transaction_id
	`

	return wsql.Query{Query: ackQuery, Args: []any{params.ConsumerGroup, params.Topic, params.LastRow.Offset, params.LastRow.ExtraData["transaction_id"]}}, nil
}

func (PostgreSQLOffsetsAdapter) ConsumedMessageQuery(_ wsql.ConsumedMessageQueryParams) (wsql.Query, error) {
	return wsql.Query{}, nil
}

func (PostgreSQLOffsetsAdapter) BeforeSubscribingQueries(params wsql.BeforeSubscribingQueriesParams) ([]wsql.Query, error) {
	return []wsql.Query{
		{
			Query: `
				INSERT INTO ` + offsetsTableName + ` (consumer_group, topic, offset_acked, last_processed_transaction_id)
				VALUES ($1, $2, 0, '0')
				ON CONFLICT DO NOTHING;
			`,
			Args: []any{params.ConsumerGroup, params.Topic},
		},
	}, nil
}
