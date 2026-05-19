package meshtasticapi

import (
	"testing"

	meshtastic "gitplac.si/gomeshtastic/protobuf/v2"
	"google.golang.org/protobuf/proto"
)

func TestDecodeTracerouteUsesReplyID(t *testing.T) {
	payload, err := proto.Marshal(&meshtastic.RouteDiscovery{
		Route:      []uint32{0x11111111, 0x22222222},
		SnrTowards: []int32{16, -128},
		RouteBack:  []uint32{0x33333333},
		SnrBack:    []int32{8},
	})
	if err != nil {
		t.Fatal(err)
	}

	packet := &meshtastic.MeshPacket{
		From: 0x01020304,
		To:   0x0a0b0c0d,
		Id:   0xdeadbeef,
		PayloadVariant: &meshtastic.MeshPacket_Decoded{
			Decoded: &meshtastic.Data{
				Portnum:   meshtastic.PortNum_TRACEROUTE_APP,
				Payload:   payload,
				ReplyId:   0x12345678,
				RequestId: 0x87654321,
			},
		},
	}

	event := decodeMeshPacket(packet)
	if event.Type != EventTraceRoute {
		t.Fatalf("expected %q, got %q", EventTraceRoute, event.Type)
	}
	if event.TraceRoute == nil {
		t.Fatal("expected decoded traceroute payload")
	}
	if event.TraceRoute.RequestID != 0x12345678 {
		t.Fatalf("expected reply_id to win, got %08x", event.TraceRoute.RequestID)
	}
	if event.TraceRoute.RxRSSI == nil || *event.TraceRoute.RxRSSI != 0 {
		t.Fatalf("expected rssi pointer value 0, got %#v", event.TraceRoute.RxRSSI)
	}
	if event.TraceRoute.RxSNR == nil || *event.TraceRoute.RxSNR != 0 {
		t.Fatalf("expected snr pointer value 0, got %#v", event.TraceRoute.RxSNR)
	}
}

func TestDecodeTracerouteFallsBackToRequestThenPacketID(t *testing.T) {
	cases := []struct {
		name      string
		replyID   uint32
		requestID uint32
		packetID  uint32
		want      uint32
	}{
		{name: "request id fallback", requestID: 0x01010101, packetID: 0x02020202, want: 0x01010101},
		{name: "packet id fallback", packetID: 0x03030303, want: 0x03030303},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := proto.Marshal(&meshtastic.RouteDiscovery{})
			if err != nil {
				t.Fatal(err)
			}
			packet := &meshtastic.MeshPacket{
				Id: tc.packetID,
				PayloadVariant: &meshtastic.MeshPacket_Decoded{
					Decoded: &meshtastic.Data{
						Portnum:   meshtastic.PortNum_TRACEROUTE_APP,
						Payload:   payload,
						ReplyId:   tc.replyID,
						RequestId: tc.requestID,
					},
				},
			}
			event := decodeMeshPacket(packet)
			if event.TraceRoute == nil {
				t.Fatal("expected decoded traceroute payload")
			}
			if event.TraceRoute.RequestID != tc.want {
				t.Fatalf("expected %08x, got %08x", tc.want, event.TraceRoute.RequestID)
			}
			if event.TraceRoute.RxRSSI == nil || event.TraceRoute.RxSNR == nil {
				t.Fatal("expected route rx metrics pointers")
			}
		})
	}
}

func TestDecodeRoutingAppRouteReplyEmitsTracerouteEvent(t *testing.T) {
	routingPayload, err := proto.Marshal(&meshtastic.Routing{
		Variant: &meshtastic.Routing_RouteReply{
			RouteReply: &meshtastic.RouteDiscovery{
				Route:      []uint32{0x11112222},
				SnrTowards: []int32{12},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	packet := &meshtastic.MeshPacket{
		From: 0x01010101,
		To:   0x02020202,
		Id:   0xabcdef01,
		PayloadVariant: &meshtastic.MeshPacket_Decoded{
			Decoded: &meshtastic.Data{
				Portnum: meshtastic.PortNum_ROUTING_APP,
				Payload: routingPayload,
				ReplyId: 0x0000beef,
			},
		},
	}

	event := decodeMeshPacket(packet)
	if event.Type != EventTraceRoute {
		t.Fatalf("expected %q, got %q", EventTraceRoute, event.Type)
	}
	if event.TraceRoute == nil {
		t.Fatal("expected decoded traceroute payload")
	}
	if event.TraceRoute.RequestID != 0x0000beef {
		t.Fatalf("expected request id 0000beef, got %08x", event.TraceRoute.RequestID)
	}
	if len(event.TraceRoute.Route) != 1 || event.TraceRoute.Route[0] != 0x11112222 {
		t.Fatalf("unexpected route payload: %#v", event.TraceRoute.Route)
	}
}
