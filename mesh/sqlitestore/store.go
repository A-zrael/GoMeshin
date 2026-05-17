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

CREATE TABLE IF NOT EXISTS positions (
	node_num INTEGER PRIMARY KEY,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	latitude REAL NOT NULL,
	longitude REAL NOT NULL,
	altitude INTEGER,
	altitude_hae INTEGER,
	ground_speed INTEGER,
	ground_track INTEGER,
	sats_in_view INTEGER NOT NULL,
	precision_bits INTEGER NOT NULL,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS positions_received_at_idx ON positions(received_at);

CREATE TABLE IF NOT EXISTS environment_telemetry (
	node_num INTEGER PRIMARY KEY,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	temperature REAL,
	relative_humidity REAL,
	barometric_pressure REAL,
	gas_resistance REAL,
	voltage REAL,
	current REAL,
	iaq INTEGER,
	distance REAL,
	lux REAL,
	white_lux REAL,
	ir_lux REAL,
	uv_lux REAL,
	wind_direction INTEGER,
	wind_speed REAL,
	wind_gust REAL,
	wind_lull REAL,
	weight REAL,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS environment_telemetry_received_at_idx ON environment_telemetry(received_at);
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

func (s *Store) SavePosition(ctx context.Context, position mesh.Position) error {
	receivedAt := position.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := position.Timestamp
	if timestamp.IsZero() {
		timestamp = receivedAt
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO positions (
	node_num, node_id, node_long_name, node_short_name, latitude, longitude,
	altitude, altitude_hae, ground_speed, ground_track, sats_in_view,
	precision_bits, timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(node_num) DO UPDATE SET
	node_id = excluded.node_id,
	node_long_name = excluded.node_long_name,
	node_short_name = excluded.node_short_name,
	latitude = excluded.latitude,
	longitude = excluded.longitude,
	altitude = excluded.altitude,
	altitude_hae = excluded.altitude_hae,
	ground_speed = excluded.ground_speed,
	ground_track = excluded.ground_track,
	sats_in_view = excluded.sats_in_view,
	precision_bits = excluded.precision_bits,
	timestamp = excluded.timestamp,
	received_at = excluded.received_at`,
		position.Node.Num,
		position.Node.ID,
		position.Node.LongName,
		position.Node.ShortName,
		position.Latitude,
		position.Longitude,
		nullableInt32(position.Altitude),
		nullableInt32(position.AltitudeHAE),
		nullableUint32(position.GroundSpeed),
		nullableUint32(position.GroundTrack),
		position.SatsInView,
		position.PrecisionBits,
		timestamp.Unix(),
		receivedAt.Unix(),
	)
	return err
}

func (s *Store) SaveEnvironmentTelemetry(ctx context.Context, environment mesh.EnvironmentTelemetry) error {
	receivedAt := environment.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := environment.Timestamp
	if timestamp.IsZero() {
		timestamp = receivedAt
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO environment_telemetry (
	node_num, node_id, node_long_name, node_short_name,
	temperature, relative_humidity, barometric_pressure, gas_resistance,
	voltage, current, iaq, distance, lux, white_lux, ir_lux, uv_lux,
	wind_direction, wind_speed, wind_gust, wind_lull, weight,
	timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(node_num) DO UPDATE SET
	node_id = excluded.node_id,
	node_long_name = excluded.node_long_name,
	node_short_name = excluded.node_short_name,
	temperature = excluded.temperature,
	relative_humidity = excluded.relative_humidity,
	barometric_pressure = excluded.barometric_pressure,
	gas_resistance = excluded.gas_resistance,
	voltage = excluded.voltage,
	current = excluded.current,
	iaq = excluded.iaq,
	distance = excluded.distance,
	lux = excluded.lux,
	white_lux = excluded.white_lux,
	ir_lux = excluded.ir_lux,
	uv_lux = excluded.uv_lux,
	wind_direction = excluded.wind_direction,
	wind_speed = excluded.wind_speed,
	wind_gust = excluded.wind_gust,
	wind_lull = excluded.wind_lull,
	weight = excluded.weight,
	timestamp = excluded.timestamp,
	received_at = excluded.received_at`,
		environment.Node.Num,
		environment.Node.ID,
		environment.Node.LongName,
		environment.Node.ShortName,
		nullableFloat32(environment.Temperature),
		nullableFloat32(environment.RelativeHumidity),
		nullableFloat32(environment.BarometricPressure),
		nullableFloat32(environment.GasResistance),
		nullableFloat32(environment.Voltage),
		nullableFloat32(environment.Current),
		nullableUint32(environment.IAQ),
		nullableFloat32(environment.Distance),
		nullableFloat32(environment.Lux),
		nullableFloat32(environment.WhiteLux),
		nullableFloat32(environment.IRLux),
		nullableFloat32(environment.UVLux),
		nullableUint32(environment.WindDirection),
		nullableFloat32(environment.WindSpeed),
		nullableFloat32(environment.WindGust),
		nullableFloat32(environment.WindLull),
		nullableFloat32(environment.Weight),
		timestamp.Unix(),
		receivedAt.Unix(),
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
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	positions, err := s.Positions(ctx)
	if err != nil {
		return nil, err
	}
	positionsByNode := make(map[uint32]mesh.Position, len(positions))
	for _, position := range positions {
		positionsByNode[position.Node.Num] = position
	}
	for index := range nodes {
		if position, ok := positionsByNode[nodes[index].Num]; ok {
			nodes[index].Position = &position
		}
	}

	environments, err := s.EnvironmentTelemetries(ctx)
	if err != nil {
		return nil, err
	}
	environmentsByNode := make(map[uint32]mesh.EnvironmentTelemetry, len(environments))
	for _, environment := range environments {
		environmentsByNode[environment.Node.Num] = environment
	}
	for index := range nodes {
		if environment, ok := environmentsByNode[nodes[index].Num]; ok {
			nodes[index].Environment = &environment
		}
	}

	return nodes, nil
}

func (s *Store) Positions(ctx context.Context) ([]mesh.Position, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT node_num, node_id, node_long_name, node_short_name, latitude, longitude,
	altitude, altitude_hae, ground_speed, ground_track, sats_in_view,
	precision_bits, timestamp, received_at
FROM positions
ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []mesh.Position
	for rows.Next() {
		var position mesh.Position
		var altitude sql.NullInt64
		var altitudeHAE sql.NullInt64
		var groundSpeed sql.NullInt64
		var groundTrack sql.NullInt64
		var timestamp int64
		var receivedAt int64
		if err := rows.Scan(
			&position.Node.Num,
			&position.Node.ID,
			&position.Node.LongName,
			&position.Node.ShortName,
			&position.Latitude,
			&position.Longitude,
			&altitude,
			&altitudeHAE,
			&groundSpeed,
			&groundTrack,
			&position.SatsInView,
			&position.PrecisionBits,
			&timestamp,
			&receivedAt,
		); err != nil {
			return nil, err
		}
		position.Altitude = int32Ptr(altitude)
		position.AltitudeHAE = int32Ptr(altitudeHAE)
		position.GroundSpeed = uint32Ptr(groundSpeed)
		position.GroundTrack = uint32Ptr(groundTrack)
		position.Timestamp = time.Unix(timestamp, 0)
		position.ReceivedAt = time.Unix(receivedAt, 0)
		positions = append(positions, position)
	}

	return positions, rows.Err()
}

func (s *Store) EnvironmentTelemetries(ctx context.Context) ([]mesh.EnvironmentTelemetry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT node_num, node_id, node_long_name, node_short_name,
	temperature, relative_humidity, barometric_pressure, gas_resistance,
	voltage, current, iaq, distance, lux, white_lux, ir_lux, uv_lux,
	wind_direction, wind_speed, wind_gust, wind_lull, weight,
	timestamp, received_at
FROM environment_telemetry
ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var environments []mesh.EnvironmentTelemetry
	for rows.Next() {
		var environment mesh.EnvironmentTelemetry
		var temperature sql.NullFloat64
		var relativeHumidity sql.NullFloat64
		var barometricPressure sql.NullFloat64
		var gasResistance sql.NullFloat64
		var voltage sql.NullFloat64
		var current sql.NullFloat64
		var iaq sql.NullInt64
		var distance sql.NullFloat64
		var lux sql.NullFloat64
		var whiteLux sql.NullFloat64
		var irLux sql.NullFloat64
		var uvLux sql.NullFloat64
		var windDirection sql.NullInt64
		var windSpeed sql.NullFloat64
		var windGust sql.NullFloat64
		var windLull sql.NullFloat64
		var weight sql.NullFloat64
		var timestamp int64
		var receivedAt int64
		if err := rows.Scan(
			&environment.Node.Num,
			&environment.Node.ID,
			&environment.Node.LongName,
			&environment.Node.ShortName,
			&temperature,
			&relativeHumidity,
			&barometricPressure,
			&gasResistance,
			&voltage,
			&current,
			&iaq,
			&distance,
			&lux,
			&whiteLux,
			&irLux,
			&uvLux,
			&windDirection,
			&windSpeed,
			&windGust,
			&windLull,
			&weight,
			&timestamp,
			&receivedAt,
		); err != nil {
			return nil, err
		}
		environment.Temperature = float32Ptr(temperature)
		environment.RelativeHumidity = float32Ptr(relativeHumidity)
		environment.BarometricPressure = float32Ptr(barometricPressure)
		environment.GasResistance = float32Ptr(gasResistance)
		environment.Voltage = float32Ptr(voltage)
		environment.Current = float32Ptr(current)
		environment.IAQ = uint32Ptr(iaq)
		environment.Distance = float32Ptr(distance)
		environment.Lux = float32Ptr(lux)
		environment.WhiteLux = float32Ptr(whiteLux)
		environment.IRLux = float32Ptr(irLux)
		environment.UVLux = float32Ptr(uvLux)
		environment.WindDirection = uint32Ptr(windDirection)
		environment.WindSpeed = float32Ptr(windSpeed)
		environment.WindGust = float32Ptr(windGust)
		environment.WindLull = float32Ptr(windLull)
		environment.Weight = float32Ptr(weight)
		environment.Timestamp = time.Unix(timestamp, 0)
		environment.ReceivedAt = time.Unix(receivedAt, 0)
		environments = append(environments, environment)
	}

	return environments, rows.Err()
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

func nullableInt32(value *int32) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullableUint32(value *uint32) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullableFloat32(value *float32) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: float64(*value), Valid: true}
}

func int32Ptr(value sql.NullInt64) *int32 {
	if !value.Valid {
		return nil
	}
	converted := int32(value.Int64)
	return &converted
}

func uint32Ptr(value sql.NullInt64) *uint32 {
	if !value.Valid {
		return nil
	}
	converted := uint32(value.Int64)
	return &converted
}

func float32Ptr(value sql.NullFloat64) *float32 {
	if !value.Valid {
		return nil
	}
	converted := float32(value.Float64)
	return &converted
}
