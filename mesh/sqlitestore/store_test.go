package sqlitestore

import (
	"context"
	"testing"
	"time"

	"meshin/mesh"
)

func TestStorePersistsMessagesNodesPositionsEnvironmentAndChannels(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir()+"/gomeshin.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	node := mesh.Node{
		Num:       0x12345678,
		ID:        "!12345678",
		LongName:  "Long Name",
		ShortName: "LN",
		LastSeen:  time.Unix(100, 0),
	}
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	altitude := int32(123)
	position := mesh.Position{
		Node:       mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		Latitude:   51.5007,
		Longitude:  -0.1246,
		Altitude:   &altitude,
		Timestamp:  time.Unix(101, 0),
		ReceivedAt: time.Unix(102, 0),
	}
	if err := store.SavePosition(ctx, position); err != nil {
		t.Fatal(err)
	}
	temperature := float32(12.5)
	humidity := float32(76.25)
	windSpeed := float32(5.5)
	environment := mesh.EnvironmentTelemetry{
		Node:             mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		Temperature:      &temperature,
		RelativeHumidity: &humidity,
		WindSpeed:        &windSpeed,
		Timestamp:        time.Unix(103, 0),
		ReceivedAt:       time.Unix(104, 0),
	}
	if err := store.SaveEnvironmentTelemetry(ctx, environment); err != nil {
		t.Fatal(err)
	}
	battery := uint32(87)
	device := mesh.DeviceTelemetry{
		Node:               mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		BatteryLevel:       &battery,
		Voltage:            &temperature,
		ChannelUtilization: &windSpeed,
		Timestamp:          time.Unix(105, 0),
		ReceivedAt:         time.Unix(106, 0),
	}
	if err := store.SaveDeviceTelemetry(ctx, device); err != nil {
		t.Fatal(err)
	}
	power := mesh.PowerTelemetry{
		Node:       mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		Ch1Voltage: &temperature,
		Timestamp:  time.Unix(107, 0),
		ReceivedAt: time.Unix(108, 0),
	}
	if err := store.SavePowerTelemetry(ctx, power); err != nil {
		t.Fatal(err)
	}
	pm25 := uint32(12)
	air := mesh.AirQualityTelemetry{
		Node:         mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		Pm25Standard: &pm25,
		Timestamp:    time.Unix(109, 0),
		ReceivedAt:   time.Unix(110, 0),
	}
	if err := store.SaveAirQualityTelemetry(ctx, air); err != nil {
		t.Fatal(err)
	}
	local := mesh.LocalStatsTelemetry{
		Node:               mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		UptimeSeconds:      123,
		ChannelUtilization: 4.2,
		AirUtilTx:          1.1,
		Timestamp:          time.Unix(111, 0),
		ReceivedAt:         time.Unix(112, 0),
	}
	if err := store.SaveLocalStatsTelemetry(ctx, local); err != nil {
		t.Fatal(err)
	}
	heart := uint32(70)
	health := mesh.HealthTelemetry{
		Node:       mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		HeartBPM:   &heart,
		Timestamp:  time.Unix(113, 0),
		ReceivedAt: time.Unix(114, 0),
	}
	if err := store.SaveHealthTelemetry(ctx, health); err != nil {
		t.Fatal(err)
	}

	channel := mesh.Channel{
		Index:    1,
		Name:     "ops",
		Role:     "SECONDARY",
		ID:       42,
		PSKBytes: 1,
	}
	if err := store.SaveChannel(ctx, channel); err != nil {
		t.Fatal(err)
	}

	message := mesh.Message{
		ID:   99,
		From: mesh.NodeRef{Num: node.Num, ID: node.ID, LongName: node.LongName, ShortName: node.ShortName},
		To:   0xffffffff,
		Channel: mesh.ChannelRef{
			Index: channel.Index,
			Name:  channel.Name,
		},
		Text:       "hello",
		RSSI:       -80,
		SNR:        7.5,
		ReceivedAt: time.Unix(123, 0),
	}
	if err := store.SaveMessage(ctx, message); err != nil {
		t.Fatal(err)
	}

	messages, err := store.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Text != "hello" {
		t.Fatalf("unexpected messages: %#v", messages)
	}

	nodes, err := store.Nodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ShortName != "LN" || nodes[0].Position == nil {
		t.Fatalf("unexpected nodes: %#v", nodes)
	}
	if nodes[0].Position.Latitude != position.Latitude || nodes[0].Position.Longitude != position.Longitude {
		t.Fatalf("unexpected node position: %#v", nodes[0].Position)
	}
	if nodes[0].Environment == nil || nodes[0].Environment.Temperature == nil || *nodes[0].Environment.Temperature != temperature {
		t.Fatalf("unexpected node environment: %#v", nodes[0].Environment)
	}

	positions, err := store.Positions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 1 || positions[0].Node.Num != node.Num || positions[0].Altitude == nil || *positions[0].Altitude != altitude {
		t.Fatalf("unexpected positions: %#v", positions)
	}

	environments, err := store.EnvironmentTelemetries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(environments) != 1 || environments[0].Node.Num != node.Num || environments[0].RelativeHumidity == nil || *environments[0].RelativeHumidity != humidity {
		t.Fatalf("unexpected environment telemetry: %#v", environments)
	}
	devices, err := store.DeviceTelemetries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].BatteryLevel == nil || *devices[0].BatteryLevel != battery {
		t.Fatalf("unexpected device telemetry: %#v", devices)
	}
	powers, err := store.PowerTelemetries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(powers) != 1 || powers[0].Ch1Voltage == nil || *powers[0].Ch1Voltage != temperature {
		t.Fatalf("unexpected power telemetry: %#v", powers)
	}
	airs, err := store.AirQualityTelemetries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(airs) != 1 || airs[0].Pm25Standard == nil || *airs[0].Pm25Standard != pm25 {
		t.Fatalf("unexpected air quality telemetry: %#v", airs)
	}
	locals, err := store.LocalStatsTelemetries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(locals) != 1 || locals[0].UptimeSeconds != local.UptimeSeconds {
		t.Fatalf("unexpected local stats telemetry: %#v", locals)
	}
	healths, err := store.HealthTelemetries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(healths) != 1 || healths[0].HeartBPM == nil || *healths[0].HeartBPM != heart {
		t.Fatalf("unexpected health telemetry: %#v", healths)
	}

	channels, err := store.Channels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].Name != "ops" {
		t.Fatalf("unexpected channels: %#v", channels)
	}
}
