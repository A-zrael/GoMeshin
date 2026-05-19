package sqlitestore

import (
	"context"
	"database/sql"
	"encoding/json"
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
CREATE TABLE IF NOT EXISTS environment_telemetry_history (
	node_num INTEGER NOT NULL,
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
CREATE INDEX IF NOT EXISTS environment_telemetry_history_node_received_idx ON environment_telemetry_history(node_num, received_at DESC);

CREATE TABLE IF NOT EXISTS device_telemetry (
	node_num INTEGER PRIMARY KEY,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	battery_level INTEGER,
	voltage REAL,
	channel_utilization REAL,
	air_util_tx REAL,
	uptime_seconds INTEGER,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS device_telemetry_received_at_idx ON device_telemetry(received_at);
CREATE TABLE IF NOT EXISTS device_telemetry_history (
	node_num INTEGER NOT NULL,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	battery_level INTEGER,
	voltage REAL,
	channel_utilization REAL,
	air_util_tx REAL,
	uptime_seconds INTEGER,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS device_telemetry_history_node_received_idx ON device_telemetry_history(node_num, received_at DESC);

CREATE TABLE IF NOT EXISTS power_telemetry (
	node_num INTEGER PRIMARY KEY,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	ch1_voltage REAL,
	ch1_current REAL,
	ch2_voltage REAL,
	ch2_current REAL,
	ch3_voltage REAL,
	ch3_current REAL,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS power_telemetry_received_at_idx ON power_telemetry(received_at);

CREATE TABLE IF NOT EXISTS air_quality_telemetry (
	node_num INTEGER PRIMARY KEY,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	pm10_standard INTEGER,
	pm25_standard INTEGER,
	pm100_standard INTEGER,
	pm10_environmental INTEGER,
	pm25_environmental INTEGER,
	pm100_environmental INTEGER,
	particles_03um INTEGER,
	particles_05um INTEGER,
	particles_10um INTEGER,
	particles_25um INTEGER,
	particles_50um INTEGER,
	particles_100um INTEGER,
	co2 INTEGER,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS air_quality_telemetry_received_at_idx ON air_quality_telemetry(received_at);

CREATE TABLE IF NOT EXISTS local_stats_telemetry (
	node_num INTEGER PRIMARY KEY,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	uptime_seconds INTEGER NOT NULL,
	channel_utilization REAL NOT NULL,
	air_util_tx REAL NOT NULL,
	num_packets_tx INTEGER NOT NULL,
	num_packets_rx INTEGER NOT NULL,
	num_packets_rx_bad INTEGER NOT NULL,
	num_online_nodes INTEGER NOT NULL,
	num_total_nodes INTEGER NOT NULL,
	num_rx_dupe INTEGER NOT NULL,
	num_tx_relay INTEGER NOT NULL,
	num_tx_relay_canceled INTEGER NOT NULL,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS local_stats_telemetry_received_at_idx ON local_stats_telemetry(received_at);
CREATE TABLE IF NOT EXISTS local_stats_telemetry_history (
	node_num INTEGER NOT NULL,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	uptime_seconds INTEGER NOT NULL,
	channel_utilization REAL NOT NULL,
	air_util_tx REAL NOT NULL,
	num_packets_tx INTEGER NOT NULL,
	num_packets_rx INTEGER NOT NULL,
	num_packets_rx_bad INTEGER NOT NULL,
	num_online_nodes INTEGER NOT NULL,
	num_total_nodes INTEGER NOT NULL,
	num_rx_dupe INTEGER NOT NULL,
	num_tx_relay INTEGER NOT NULL,
	num_tx_relay_canceled INTEGER NOT NULL,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS local_stats_telemetry_history_node_received_idx ON local_stats_telemetry_history(node_num, received_at DESC);

CREATE TABLE IF NOT EXISTS health_telemetry (
	node_num INTEGER PRIMARY KEY,
	node_id TEXT NOT NULL,
	node_long_name TEXT NOT NULL,
	node_short_name TEXT NOT NULL,
	heart_bpm INTEGER,
	spo2 INTEGER,
	temperature REAL,
	timestamp INTEGER NOT NULL,
	received_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS health_telemetry_received_at_idx ON health_telemetry(received_at);

CREATE TABLE IF NOT EXISTS trace_routes (
	request_id INTEGER NOT NULL,
	from_num INTEGER NOT NULL,
	to_num INTEGER NOT NULL,
	received_at INTEGER NOT NULL,
	payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS trace_routes_received_at_idx ON trace_routes(received_at DESC);
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

func (s *Store) SaveTraceRoute(ctx context.Context, route mesh.TraceRoute) error {
	receivedAt := route.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	payload, err := json.Marshal(route)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO trace_routes (
	request_id, from_num, to_num, received_at, payload_json
) VALUES (?, ?, ?, ?, ?)`,
		route.RequestID,
		route.From,
		route.To,
		receivedAt.Unix(),
		string(payload),
	)
	if err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `
DELETE FROM trace_routes
WHERE rowid NOT IN (
	SELECT rowid FROM trace_routes ORDER BY received_at DESC LIMIT 5000
)`)
	return nil
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
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO environment_telemetry_history (
	node_num, node_id, node_long_name, node_short_name,
	temperature, relative_humidity, barometric_pressure, gas_resistance,
	voltage, current, iaq, distance, lux, white_lux, ir_lux, uv_lux,
	wind_direction, wind_speed, wind_gust, wind_lull, weight,
	timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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

func (s *Store) SaveDeviceTelemetry(ctx context.Context, sample mesh.DeviceTelemetry) error {
	receivedAt := sample.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := sample.Timestamp
	if timestamp.IsZero() {
		timestamp = receivedAt
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO device_telemetry (
	node_num, node_id, node_long_name, node_short_name,
	battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds, timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(node_num) DO UPDATE SET
	node_id = excluded.node_id,
	node_long_name = excluded.node_long_name,
	node_short_name = excluded.node_short_name,
	battery_level = excluded.battery_level,
	voltage = excluded.voltage,
	channel_utilization = excluded.channel_utilization,
	air_util_tx = excluded.air_util_tx,
	uptime_seconds = excluded.uptime_seconds,
	timestamp = excluded.timestamp,
	received_at = excluded.received_at`,
		sample.Node.Num, sample.Node.ID, sample.Node.LongName, sample.Node.ShortName,
		nullableUint32(sample.BatteryLevel), nullableFloat32(sample.Voltage), nullableFloat32(sample.ChannelUtilization),
		nullableFloat32(sample.AirUtilTx), nullableUint32(sample.UptimeSeconds), timestamp.Unix(), receivedAt.Unix(),
	)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO device_telemetry_history (
	node_num, node_id, node_long_name, node_short_name,
	battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds, timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.Node.Num, sample.Node.ID, sample.Node.LongName, sample.Node.ShortName,
		nullableUint32(sample.BatteryLevel), nullableFloat32(sample.Voltage), nullableFloat32(sample.ChannelUtilization),
		nullableFloat32(sample.AirUtilTx), nullableUint32(sample.UptimeSeconds), timestamp.Unix(), receivedAt.Unix(),
	)
	return err
}

func (s *Store) SavePowerTelemetry(ctx context.Context, sample mesh.PowerTelemetry) error {
	receivedAt := sample.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := sample.Timestamp
	if timestamp.IsZero() {
		timestamp = receivedAt
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO power_telemetry (
	node_num, node_id, node_long_name, node_short_name,
	ch1_voltage, ch1_current, ch2_voltage, ch2_current, ch3_voltage, ch3_current,
	timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(node_num) DO UPDATE SET
	node_id = excluded.node_id,
	node_long_name = excluded.node_long_name,
	node_short_name = excluded.node_short_name,
	ch1_voltage = excluded.ch1_voltage,
	ch1_current = excluded.ch1_current,
	ch2_voltage = excluded.ch2_voltage,
	ch2_current = excluded.ch2_current,
	ch3_voltage = excluded.ch3_voltage,
	ch3_current = excluded.ch3_current,
	timestamp = excluded.timestamp,
	received_at = excluded.received_at`,
		sample.Node.Num, sample.Node.ID, sample.Node.LongName, sample.Node.ShortName,
		nullableFloat32(sample.Ch1Voltage), nullableFloat32(sample.Ch1Current), nullableFloat32(sample.Ch2Voltage),
		nullableFloat32(sample.Ch2Current), nullableFloat32(sample.Ch3Voltage), nullableFloat32(sample.Ch3Current),
		timestamp.Unix(), receivedAt.Unix(),
	)
	return err
}

func (s *Store) SaveAirQualityTelemetry(ctx context.Context, sample mesh.AirQualityTelemetry) error {
	receivedAt := sample.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := sample.Timestamp
	if timestamp.IsZero() {
		timestamp = receivedAt
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO air_quality_telemetry (
	node_num, node_id, node_long_name, node_short_name,
	pm10_standard, pm25_standard, pm100_standard, pm10_environmental, pm25_environmental, pm100_environmental,
	particles_03um, particles_05um, particles_10um, particles_25um, particles_50um, particles_100um, co2,
	timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(node_num) DO UPDATE SET
	node_id = excluded.node_id,
	node_long_name = excluded.node_long_name,
	node_short_name = excluded.node_short_name,
	pm10_standard = excluded.pm10_standard,
	pm25_standard = excluded.pm25_standard,
	pm100_standard = excluded.pm100_standard,
	pm10_environmental = excluded.pm10_environmental,
	pm25_environmental = excluded.pm25_environmental,
	pm100_environmental = excluded.pm100_environmental,
	particles_03um = excluded.particles_03um,
	particles_05um = excluded.particles_05um,
	particles_10um = excluded.particles_10um,
	particles_25um = excluded.particles_25um,
	particles_50um = excluded.particles_50um,
	particles_100um = excluded.particles_100um,
	co2 = excluded.co2,
	timestamp = excluded.timestamp,
	received_at = excluded.received_at`,
		sample.Node.Num, sample.Node.ID, sample.Node.LongName, sample.Node.ShortName,
		nullableUint32(sample.Pm10Standard), nullableUint32(sample.Pm25Standard), nullableUint32(sample.Pm100Standard),
		nullableUint32(sample.Pm10Environmental), nullableUint32(sample.Pm25Environmental), nullableUint32(sample.Pm100Environmental),
		nullableUint32(sample.Particles03um), nullableUint32(sample.Particles05um), nullableUint32(sample.Particles10um),
		nullableUint32(sample.Particles25um), nullableUint32(sample.Particles50um), nullableUint32(sample.Particles100um),
		nullableUint32(sample.CO2), timestamp.Unix(), receivedAt.Unix(),
	)
	return err
}

func (s *Store) SaveLocalStatsTelemetry(ctx context.Context, sample mesh.LocalStatsTelemetry) error {
	receivedAt := sample.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := sample.Timestamp
	if timestamp.IsZero() {
		timestamp = receivedAt
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO local_stats_telemetry (
	node_num, node_id, node_long_name, node_short_name,
	uptime_seconds, channel_utilization, air_util_tx, num_packets_tx, num_packets_rx, num_packets_rx_bad,
	num_online_nodes, num_total_nodes, num_rx_dupe, num_tx_relay, num_tx_relay_canceled,
	timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(node_num) DO UPDATE SET
	node_id = excluded.node_id,
	node_long_name = excluded.node_long_name,
	node_short_name = excluded.node_short_name,
	uptime_seconds = excluded.uptime_seconds,
	channel_utilization = excluded.channel_utilization,
	air_util_tx = excluded.air_util_tx,
	num_packets_tx = excluded.num_packets_tx,
	num_packets_rx = excluded.num_packets_rx,
	num_packets_rx_bad = excluded.num_packets_rx_bad,
	num_online_nodes = excluded.num_online_nodes,
	num_total_nodes = excluded.num_total_nodes,
	num_rx_dupe = excluded.num_rx_dupe,
	num_tx_relay = excluded.num_tx_relay,
	num_tx_relay_canceled = excluded.num_tx_relay_canceled,
	timestamp = excluded.timestamp,
	received_at = excluded.received_at`,
		sample.Node.Num, sample.Node.ID, sample.Node.LongName, sample.Node.ShortName,
		sample.UptimeSeconds, sample.ChannelUtilization, sample.AirUtilTx, sample.NumPacketsTx, sample.NumPacketsRx, sample.NumPacketsRxBad,
		sample.NumOnlineNodes, sample.NumTotalNodes, sample.NumRxDupe, sample.NumTxRelay, sample.NumTxRelayCanceled,
		timestamp.Unix(), receivedAt.Unix(),
	)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO local_stats_telemetry_history (
	node_num, node_id, node_long_name, node_short_name,
	uptime_seconds, channel_utilization, air_util_tx, num_packets_tx, num_packets_rx, num_packets_rx_bad,
	num_online_nodes, num_total_nodes, num_rx_dupe, num_tx_relay, num_tx_relay_canceled,
	timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.Node.Num, sample.Node.ID, sample.Node.LongName, sample.Node.ShortName,
		sample.UptimeSeconds, sample.ChannelUtilization, sample.AirUtilTx, sample.NumPacketsTx, sample.NumPacketsRx, sample.NumPacketsRxBad,
		sample.NumOnlineNodes, sample.NumTotalNodes, sample.NumRxDupe, sample.NumTxRelay, sample.NumTxRelayCanceled,
		timestamp.Unix(), receivedAt.Unix(),
	)
	return err
}

func (s *Store) SaveHealthTelemetry(ctx context.Context, sample mesh.HealthTelemetry) error {
	receivedAt := sample.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	timestamp := sample.Timestamp
	if timestamp.IsZero() {
		timestamp = receivedAt
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO health_telemetry (
	node_num, node_id, node_long_name, node_short_name, heart_bpm, spo2, temperature, timestamp, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(node_num) DO UPDATE SET
	node_id = excluded.node_id,
	node_long_name = excluded.node_long_name,
	node_short_name = excluded.node_short_name,
	heart_bpm = excluded.heart_bpm,
	spo2 = excluded.spo2,
	temperature = excluded.temperature,
	timestamp = excluded.timestamp,
	received_at = excluded.received_at`,
		sample.Node.Num, sample.Node.ID, sample.Node.LongName, sample.Node.ShortName,
		nullableUint32(sample.HeartBPM), nullableUint32(sample.SpO2), nullableFloat32(sample.Temperature),
		timestamp.Unix(), receivedAt.Unix(),
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

func (s *Store) DeviceTelemetries(ctx context.Context) ([]mesh.DeviceTelemetry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT node_num, node_id, node_long_name, node_short_name, battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds, timestamp, received_at
FROM device_telemetry ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.DeviceTelemetry
	for rows.Next() {
		var sample mesh.DeviceTelemetry
		var battery, uptime sql.NullInt64
		var voltage, channelUtil, airUtil sql.NullFloat64
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &battery, &voltage, &channelUtil, &airUtil, &uptime, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.BatteryLevel = uint32Ptr(battery)
		sample.Voltage = float32Ptr(voltage)
		sample.ChannelUtilization = float32Ptr(channelUtil)
		sample.AirUtilTx = float32Ptr(airUtil)
		sample.UptimeSeconds = uint32Ptr(uptime)
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
}

func (s *Store) PowerTelemetries(ctx context.Context) ([]mesh.PowerTelemetry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT node_num, node_id, node_long_name, node_short_name, ch1_voltage, ch1_current, ch2_voltage, ch2_current, ch3_voltage, ch3_current, timestamp, received_at
FROM power_telemetry ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.PowerTelemetry
	for rows.Next() {
		var sample mesh.PowerTelemetry
		var ch1v, ch1c, ch2v, ch2c, ch3v, ch3c sql.NullFloat64
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &ch1v, &ch1c, &ch2v, &ch2c, &ch3v, &ch3c, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.Ch1Voltage = float32Ptr(ch1v)
		sample.Ch1Current = float32Ptr(ch1c)
		sample.Ch2Voltage = float32Ptr(ch2v)
		sample.Ch2Current = float32Ptr(ch2c)
		sample.Ch3Voltage = float32Ptr(ch3v)
		sample.Ch3Current = float32Ptr(ch3c)
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
}

func (s *Store) AirQualityTelemetries(ctx context.Context) ([]mesh.AirQualityTelemetry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT node_num, node_id, node_long_name, node_short_name, pm10_standard, pm25_standard, pm100_standard, pm10_environmental, pm25_environmental, pm100_environmental, particles_03um, particles_05um, particles_10um, particles_25um, particles_50um, particles_100um, co2, timestamp, received_at
FROM air_quality_telemetry ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.AirQualityTelemetry
	for rows.Next() {
		var sample mesh.AirQualityTelemetry
		var pm10s, pm25s, pm100s, pm10e, pm25e, pm100e, p03, p05, p10, p25, p50, p100, co2 sql.NullInt64
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &pm10s, &pm25s, &pm100s, &pm10e, &pm25e, &pm100e, &p03, &p05, &p10, &p25, &p50, &p100, &co2, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.Pm10Standard = uint32Ptr(pm10s)
		sample.Pm25Standard = uint32Ptr(pm25s)
		sample.Pm100Standard = uint32Ptr(pm100s)
		sample.Pm10Environmental = uint32Ptr(pm10e)
		sample.Pm25Environmental = uint32Ptr(pm25e)
		sample.Pm100Environmental = uint32Ptr(pm100e)
		sample.Particles03um = uint32Ptr(p03)
		sample.Particles05um = uint32Ptr(p05)
		sample.Particles10um = uint32Ptr(p10)
		sample.Particles25um = uint32Ptr(p25)
		sample.Particles50um = uint32Ptr(p50)
		sample.Particles100um = uint32Ptr(p100)
		sample.CO2 = uint32Ptr(co2)
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
}

func (s *Store) LocalStatsTelemetries(ctx context.Context) ([]mesh.LocalStatsTelemetry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT node_num, node_id, node_long_name, node_short_name, uptime_seconds, channel_utilization, air_util_tx, num_packets_tx, num_packets_rx, num_packets_rx_bad, num_online_nodes, num_total_nodes, num_rx_dupe, num_tx_relay, num_tx_relay_canceled, timestamp, received_at
FROM local_stats_telemetry ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.LocalStatsTelemetry
	for rows.Next() {
		var sample mesh.LocalStatsTelemetry
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &sample.UptimeSeconds, &sample.ChannelUtilization, &sample.AirUtilTx, &sample.NumPacketsTx, &sample.NumPacketsRx, &sample.NumPacketsRxBad, &sample.NumOnlineNodes, &sample.NumTotalNodes, &sample.NumRxDupe, &sample.NumTxRelay, &sample.NumTxRelayCanceled, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
}

func (s *Store) HealthTelemetries(ctx context.Context) ([]mesh.HealthTelemetry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT node_num, node_id, node_long_name, node_short_name, heart_bpm, spo2, temperature, timestamp, received_at
FROM health_telemetry ORDER BY received_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.HealthTelemetry
	for rows.Next() {
		var sample mesh.HealthTelemetry
		var heart, spo2 sql.NullInt64
		var temp sql.NullFloat64
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &heart, &spo2, &temp, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.HeartBPM = uint32Ptr(heart)
		sample.SpO2 = uint32Ptr(spo2)
		sample.Temperature = float32Ptr(temp)
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
}

func (s *Store) EnvironmentTelemetryHistory(ctx context.Context, nodeNum uint32, limit int) ([]mesh.EnvironmentTelemetry, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `
SELECT node_num, node_id, node_long_name, node_short_name,
	temperature, relative_humidity, barometric_pressure, gas_resistance,
	voltage, current, iaq, distance, lux, white_lux, ir_lux, uv_lux,
	wind_direction, wind_speed, wind_gust, wind_lull, weight,
	timestamp, received_at
FROM environment_telemetry_history`
	args := make([]interface{}, 0, 2)
	if nodeNum != 0 {
		query += ` WHERE node_num = ?`
		args = append(args, nodeNum)
	}
	query += ` ORDER BY received_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.EnvironmentTelemetry
	for rows.Next() {
		var sample mesh.EnvironmentTelemetry
		var temperature, relativeHumidity, barometricPressure, gasResistance, voltage, current, distance, lux, whiteLux, irLux, uvLux, windSpeed, windGust, windLull, weight sql.NullFloat64
		var iaq, windDirection sql.NullInt64
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &temperature, &relativeHumidity, &barometricPressure, &gasResistance, &voltage, &current, &iaq, &distance, &lux, &whiteLux, &irLux, &uvLux, &windDirection, &windSpeed, &windGust, &windLull, &weight, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.Temperature = float32Ptr(temperature)
		sample.RelativeHumidity = float32Ptr(relativeHumidity)
		sample.BarometricPressure = float32Ptr(barometricPressure)
		sample.GasResistance = float32Ptr(gasResistance)
		sample.Voltage = float32Ptr(voltage)
		sample.Current = float32Ptr(current)
		sample.IAQ = uint32Ptr(iaq)
		sample.Distance = float32Ptr(distance)
		sample.Lux = float32Ptr(lux)
		sample.WhiteLux = float32Ptr(whiteLux)
		sample.IRLux = float32Ptr(irLux)
		sample.UVLux = float32Ptr(uvLux)
		sample.WindDirection = uint32Ptr(windDirection)
		sample.WindSpeed = float32Ptr(windSpeed)
		sample.WindGust = float32Ptr(windGust)
		sample.WindLull = float32Ptr(windLull)
		sample.Weight = float32Ptr(weight)
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
}

func (s *Store) DeviceTelemetryHistory(ctx context.Context, nodeNum uint32, limit int) ([]mesh.DeviceTelemetry, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT node_num, node_id, node_long_name, node_short_name, battery_level, voltage, channel_utilization, air_util_tx, uptime_seconds, timestamp, received_at FROM device_telemetry_history`
	args := make([]interface{}, 0, 2)
	if nodeNum != 0 {
		query += ` WHERE node_num = ?`
		args = append(args, nodeNum)
	}
	query += ` ORDER BY received_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.DeviceTelemetry
	for rows.Next() {
		var sample mesh.DeviceTelemetry
		var battery, uptime sql.NullInt64
		var voltage, channelUtil, airUtil sql.NullFloat64
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &battery, &voltage, &channelUtil, &airUtil, &uptime, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.BatteryLevel = uint32Ptr(battery)
		sample.Voltage = float32Ptr(voltage)
		sample.ChannelUtilization = float32Ptr(channelUtil)
		sample.AirUtilTx = float32Ptr(airUtil)
		sample.UptimeSeconds = uint32Ptr(uptime)
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
}

func (s *Store) LocalStatsTelemetryHistory(ctx context.Context, nodeNum uint32, limit int) ([]mesh.LocalStatsTelemetry, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT node_num, node_id, node_long_name, node_short_name, uptime_seconds, channel_utilization, air_util_tx, num_packets_tx, num_packets_rx, num_packets_rx_bad, num_online_nodes, num_total_nodes, num_rx_dupe, num_tx_relay, num_tx_relay_canceled, timestamp, received_at FROM local_stats_telemetry_history`
	args := make([]interface{}, 0, 2)
	if nodeNum != 0 {
		query += ` WHERE node_num = ?`
		args = append(args, nodeNum)
	}
	query += ` ORDER BY received_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mesh.LocalStatsTelemetry
	for rows.Next() {
		var sample mesh.LocalStatsTelemetry
		var timestamp, receivedAt int64
		if err := rows.Scan(&sample.Node.Num, &sample.Node.ID, &sample.Node.LongName, &sample.Node.ShortName, &sample.UptimeSeconds, &sample.ChannelUtilization, &sample.AirUtilTx, &sample.NumPacketsTx, &sample.NumPacketsRx, &sample.NumPacketsRxBad, &sample.NumOnlineNodes, &sample.NumTotalNodes, &sample.NumRxDupe, &sample.NumTxRelay, &sample.NumTxRelayCanceled, &timestamp, &receivedAt); err != nil {
			return nil, err
		}
		sample.Timestamp = time.Unix(timestamp, 0)
		sample.ReceivedAt = time.Unix(receivedAt, 0)
		out = append(out, sample)
	}
	return out, rows.Err()
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

func (s *Store) TraceRoutes(ctx context.Context, nodeNum uint32, limit int) ([]mesh.TraceRoute, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `SELECT payload_json, received_at FROM trace_routes`
	args := []interface{}{}
	if nodeNum != 0 {
		query += ` WHERE from_num = ? OR to_num = ?`
		args = append(args, nodeNum, nodeNum)
	}
	query += ` ORDER BY received_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]mesh.TraceRoute, 0, limit)
	for rows.Next() {
		var payload string
		var receivedAt int64
		if err := rows.Scan(&payload, &receivedAt); err != nil {
			return nil, err
		}
		var route mesh.TraceRoute
		if err := json.Unmarshal([]byte(payload), &route); err != nil {
			continue
		}
		if route.ReceivedAt.IsZero() {
			route.ReceivedAt = time.Unix(receivedAt, 0)
		}
		out = append(out, route)
	}
	return out, rows.Err()
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
