package sqlitestore

import (
	"context"
	"database/sql"
	"time"

	"meshin/mesh"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER NOT NULL,
	from_num INTEGER NOT NULL,
	from_id TEXT NOT NULL,
	from_long_name TEXT NOT NULL,
	from_short_name TEXT NOT NULL,
	to_num INTEGER NOT NULL,
	channel_index INTEGER NOT NULL,
	channel_name TEXT NOT NULL,
	text TEXT NOT NULL,
	rssi INTEGER NOT NULL,
	snr REAL NOT NULL,
	received_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS messages_received_at_idx ON messages(received_at);

CREATE TABLE IF NOT EXISTS nodes (
	num INTEGER PRIMARY KEY,
	id TEXT NOT NULL,
	long_name TEXT NOT NULL,
	short_name TEXT NOT NULL,
	last_seen INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS channels (
	idx INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	role TEXT NOT NULL,
	channel_id INTEGER NOT NULL,
	psk_bytes INTEGER NOT NULL,
	uplink_enabled INTEGER NOT NULL,
	downlink_enabled INTEGER NOT NULL
);
`)
	return err
}

func (s *Store) SaveMessage(ctx context.Context, message mesh.Message) error {
	receivedAt := message.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO messages (
	id, from_num, from_id, from_long_name, from_short_name, to_num,
	channel_index, channel_name, text, rssi, snr, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID,
		message.From.Num,
		message.From.ID,
		message.From.LongName,
		message.From.ShortName,
		message.To,
		message.Channel.Index,
		message.Channel.Name,
		message.Text,
		message.RSSI,
		message.SNR,
		receivedAt.Unix(),
	)
	return err
}

func (s *Store) SaveNode(ctx context.Context, node mesh.Node) error {
	lastSeen := node.LastSeen
	if lastSeen.IsZero() {
		lastSeen = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO nodes (num, id, long_name, short_name, last_seen)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(num) DO UPDATE SET
	id = excluded.id,
	long_name = excluded.long_name,
	short_name = excluded.short_name,
	last_seen = excluded.last_seen`,
		node.Num,
		node.ID,
		node.LongName,
		node.ShortName,
		lastSeen.Unix(),
	)
	return err
}

func (s *Store) SaveChannel(ctx context.Context, channel mesh.Channel) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO channels (idx, name, role, channel_id, psk_bytes, uplink_enabled, downlink_enabled)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(idx) DO UPDATE SET
	name = excluded.name,
	role = excluded.role,
	channel_id = excluded.channel_id,
	psk_bytes = excluded.psk_bytes,
	uplink_enabled = excluded.uplink_enabled,
	downlink_enabled = excluded.downlink_enabled`,
		channel.Index,
		channel.Name,
		channel.Role,
		channel.ID,
		channel.PSKBytes,
		boolInt(channel.UplinkEnabled),
		boolInt(channel.DownlinkEnabled),
	)
	return err
}

func (s *Store) Messages(ctx context.Context) ([]mesh.Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, from_num, from_id, from_long_name, from_short_name, to_num,
	channel_index, channel_name, text, rssi, snr, received_at
FROM messages
ORDER BY received_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []mesh.Message
	for rows.Next() {
		var message mesh.Message
		var receivedAt int64
		if err := rows.Scan(
			&message.ID,
			&message.From.Num,
			&message.From.ID,
			&message.From.LongName,
			&message.From.ShortName,
			&message.To,
			&message.Channel.Index,
			&message.Channel.Name,
			&message.Text,
			&message.RSSI,
			&message.SNR,
			&receivedAt,
		); err != nil {
			return nil, err
		}
		message.ReceivedAt = time.Unix(receivedAt, 0)
		messages = append(messages, message)
	}

	return messages, rows.Err()
}

func (s *Store) Nodes(ctx context.Context) ([]mesh.Node, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT num, id, long_name, short_name, last_seen
FROM nodes
ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []mesh.Node
	for rows.Next() {
		var node mesh.Node
		var lastSeen int64
		if err := rows.Scan(&node.Num, &node.ID, &node.LongName, &node.ShortName, &lastSeen); err != nil {
			return nil, err
		}
		node.LastSeen = time.Unix(lastSeen, 0)
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

func (s *Store) Channels(ctx context.Context) ([]mesh.Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT idx, name, role, channel_id, psk_bytes, uplink_enabled, downlink_enabled
FROM channels
ORDER BY idx ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []mesh.Channel
	for rows.Next() {
		var channel mesh.Channel
		var uplink int
		var downlink int
		if err := rows.Scan(
			&channel.Index,
			&channel.Name,
			&channel.Role,
			&channel.ID,
			&channel.PSKBytes,
			&uplink,
			&downlink,
		); err != nil {
			return nil, err
		}
		channel.UplinkEnabled = uplink != 0
		channel.DownlinkEnabled = downlink != 0
		channels = append(channels, channel)
	}

	return channels, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
