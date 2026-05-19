package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"meshin/mesh"
	"meshin/mesh/sqlitestore"
)

type apiServer struct {
	mesh        *mesh.Mesh
	corsOrigins []string
	webDir      string
}

type errorResponse struct {
	Error string `json:"error"`
}

type sendRequest struct {
	Text    string  `json:"text"`
	Channel string  `json:"channel,omitempty"`
	To      nodeNum `json:"to,omitempty"`
	WantAck bool    `json:"wantAck,omitempty"`
}

type sendResponse struct {
	ID uint32 `json:"id"`
}

type channelRequest struct {
	Name string `json:"name"`
	PSK  []byte `json:"psk,omitempty"`
}

type traceRouteRequest struct {
	To             nodeNum `json:"to"`
	Channel        string  `json:"channel,omitempty"`
	HopLimit       uint32  `json:"hopLimit,omitempty"`
	TimeoutSeconds int     `json:"timeoutSeconds,omitempty"`
}

type radioSettingsUpdateRequest struct {
	HopLimit  *uint32 `json:"hopLimit,omitempty"`
	TxEnabled *bool   `json:"txEnabled,omitempty"`
	Role      *string `json:"role,omitempty"`
}

type fixedPositionRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  *int32  `json:"altitude,omitempty"`
}

type spoofTemperatureRequest struct {
	Channel     string  `json:"channel,omitempty"`
	Temperature float32 `json:"temperature"`
}

type spoofEnvironmentRequest struct {
	Channel     string   `json:"channel,omitempty"`
	Temperature *float32 `json:"temperature,omitempty"`
	Humidity    *float32 `json:"humidity,omitempty"`
	Pressure    *float32 `json:"pressure,omitempty"`
}

type spoofDeviceRequest struct {
	Channel            string   `json:"channel,omitempty"`
	BatteryLevel       *uint32  `json:"batteryLevel,omitempty"`
	Voltage            *float32 `json:"voltage,omitempty"`
	ChannelUtilization *float32 `json:"channelUtilization,omitempty"`
	AirUtilTX          *float32 `json:"airUtilTx,omitempty"`
	UptimeSeconds      *uint32  `json:"uptimeSeconds,omitempty"`
}

type spoofPowerRequest struct {
	Channel    string   `json:"channel,omitempty"`
	Ch1Voltage *float32 `json:"ch1Voltage,omitempty"`
	Ch1Current *float32 `json:"ch1Current,omitempty"`
	Ch2Voltage *float32 `json:"ch2Voltage,omitempty"`
	Ch2Current *float32 `json:"ch2Current,omitempty"`
	Ch3Voltage *float32 `json:"ch3Voltage,omitempty"`
	Ch3Current *float32 `json:"ch3Current,omitempty"`
}

type eventEnvelope struct {
	Type string      `json:"type"`
	Time time.Time   `json:"time"`
	Data interface{} `json:"data"`
}

type nodeNum uint32

const (
	defaultTraceRouteTimeout = 90 * time.Second
	minTraceRouteTimeout     = 5 * time.Second
	maxTraceRouteTimeout     = 5 * time.Minute
)

func main() {
	port := flag.String("port", "/dev/ttyUSB0", "serial port connected to the Meshtastic radio")
	baud := flag.Int("baud", 115200, "serial baud rate")
	dbPath := flag.String("db", "gomeshin.db", "SQLite database path")
	listen := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	unixSocket := flag.String("unix", "", "optional Unix socket path instead of TCP")
	corsOrigin := flag.String("cors-origin", "", "optional comma-separated Access-Control-Allow-Origin values")
	webDir := flag.String("web-dir", "", "optional directory to serve as a same-origin web client")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	store, err := sqlitestore.Open(startCtx, *dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	meshNode, err := mesh.Open(startCtx, mesh.Config{
		Port:  *port,
		Baud:  *baud,
		Store: store,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer meshNode.Close()

	api := &apiServer{
		mesh:        meshNode,
		corsOrigins: parseCORSOrigins(*corsOrigin),
		webDir:      *webDir,
	}
	server := &http.Server{
		Handler:      api.routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
	}

	errCh := make(chan error, 1)
	if *unixSocket != "" {
		_ = os.Remove(*unixSocket)
		listener, err := net.Listen("unix", *unixSocket)
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove(*unixSocket)
		log.Printf("gomeshind listening on unix socket %s", *unixSocket)
		go func() {
			errCh <- server.Serve(listener)
		}()
	} else {
		server.Addr = *listen
		log.Printf("gomeshind listening on http://%s", *listen)
		go func() {
			errCh <- server.ListenAndServe()
		}()
	}

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}
}

func (s *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/messages", s.handleMessages)
	mux.HandleFunc("/nodes", s.handleNodes)
	mux.HandleFunc("/positions", s.handlePositions)
	mux.HandleFunc("/telemetry/environment", s.handleEnvironmentTelemetry)
	mux.HandleFunc("/telemetry/device", s.handleDeviceTelemetry)
	mux.HandleFunc("/telemetry/power", s.handlePowerTelemetry)
	mux.HandleFunc("/telemetry/airquality", s.handleAirQualityTelemetry)
	mux.HandleFunc("/telemetry/localstats", s.handleLocalStatsTelemetry)
	mux.HandleFunc("/telemetry/health", s.handleHealthTelemetry)
	mux.HandleFunc("/telemetry/environment/history", s.handleEnvironmentTelemetryHistory)
	mux.HandleFunc("/telemetry/device/history", s.handleDeviceTelemetryHistory)
	mux.HandleFunc("/telemetry/localstats/history", s.handleLocalStatsTelemetryHistory)
	mux.HandleFunc("/channels", s.handleChannels)
	mux.HandleFunc("/channels/", s.handleChannel)
	mux.HandleFunc("/traceroute", s.handleTraceRoute)
	mux.HandleFunc("/traceroutes", s.handleTraceRoutes)
	mux.HandleFunc("/traceroutes/pending", s.handlePendingTraceRoutes)
	mux.HandleFunc("/settings/radio", s.handleRadioSettings)
	mux.HandleFunc("/settings/location", s.handleFixedLocation)
	mux.HandleFunc("/spoof/temperature", s.handleSpoofTemperature)
	mux.HandleFunc("/spoof/environment", s.handleSpoofEnvironment)
	mux.HandleFunc("/spoof/device", s.handleSpoofDevice)
	mux.HandleFunc("/spoof/power", s.handleSpoofPower)
	mux.HandleFunc("/events", s.handleEvents)
	if s.webDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(s.webDir)))
	}
	return s.withCommonHeaders(mux)
}

func (s *apiServer) withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := s.allowedOrigin(r.Header.Get("Origin")); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "content-type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *apiServer) allowedOrigin(origin string) string {
	if origin == "" {
		return ""
	}

	if len(s.corsOrigins) == 0 && isLoopbackOrigin(origin) {
		return origin
	}

	for _, allowed := range s.corsOrigins {
		if allowed == "*" {
			return "*"
		}
		if allowed == origin {
			return origin
		}
	}
	return ""
}

func isLoopbackOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *apiServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		messages, err := s.mesh.Messages(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		channel := r.URL.Query().Get("channel")
		if channel != "" {
			messages = filterMessages(messages, channel)
		}
		writeJSON(w, http.StatusOK, messages)
	case http.MethodPost:
		var req sendRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		req.Text = strings.TrimSpace(req.Text)
		if req.Text == "" {
			writeError(w, http.StatusBadRequest, errors.New("text is required"))
			return
		}
		id, err := s.mesh.Send(req.Text, mesh.SendOptions{
			Channel: req.Channel,
			To:      uint32(req.To),
			WantAck: req.WantAck,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, sendResponse{ID: id})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *apiServer) handleNodes(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	nodes, err := s.mesh.Nodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *apiServer) handlePositions(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	positions, err := s.mesh.Positions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, positions)
}

func (s *apiServer) handleEnvironmentTelemetry(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	environments, err := s.mesh.EnvironmentTelemetries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, environments)
}

func (s *apiServer) handleDeviceTelemetry(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	samples, err := s.mesh.DeviceTelemetries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *apiServer) handlePowerTelemetry(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	samples, err := s.mesh.PowerTelemetries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *apiServer) handleAirQualityTelemetry(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	samples, err := s.mesh.AirQualityTelemetries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *apiServer) handleLocalStatsTelemetry(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	samples, err := s.mesh.LocalStatsTelemetries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *apiServer) handleHealthTelemetry(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	samples, err := s.mesh.HealthTelemetries(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *apiServer) handleEnvironmentTelemetryHistory(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	node, limit, err := parseHistoryQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	samples, err := s.mesh.EnvironmentTelemetryHistory(r.Context(), node, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *apiServer) handleDeviceTelemetryHistory(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	node, limit, err := parseHistoryQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	samples, err := s.mesh.DeviceTelemetryHistory(r.Context(), node, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func (s *apiServer) handleLocalStatsTelemetryHistory(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	node, limit, err := parseHistoryQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	samples, err := s.mesh.LocalStatsTelemetryHistory(r.Context(), node, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func parseHistoryQuery(r *http.Request) (uint32, int, error) {
	var node uint32
	if value := strings.TrimSpace(r.URL.Query().Get("node")); value != "" {
		parsed, err := parseNodeQueryValue(value)
		if err != nil {
			return 0, 0, err
		}
		node = parsed
	}
	limit := 400
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 5000 {
			return 0, 0, errors.New("limit must be an integer between 1 and 5000")
		}
		limit = parsed
	}
	return node, limit, nil
}

func parseNodeQueryValue(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if strings.HasPrefix(value, "!") {
		parsed, err := strconv.ParseUint(strings.TrimPrefix(value, "!"), 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid node query value %q", value)
		}
		return uint32(parsed), nil
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid node query value %q", value)
	}
	return uint32(parsed), nil
}

func (s *apiServer) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channels, err := s.mesh.Channels(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, channels)
	case http.MethodPost:
		var req channelRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := s.mesh.AddChannel(ctx, mesh.ChannelOptions{Name: req.Name, PSK: req.PSK}); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		channels, err := s.mesh.Channels(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, channels)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *apiServer) handleChannel(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodDelete) {
		return
	}
	name, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/channels/"))
	if err != nil || strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, errors.New("channel name is required"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := s.mesh.RemoveChannel(ctx, name); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *apiServer) handleTraceRoute(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req traceRouteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultTraceRouteTimeout
	} else {
		if timeout < minTraceRouteTimeout {
			timeout = minTraceRouteTimeout
		}
		if timeout > maxTraceRouteTimeout {
			timeout = maxTraceRouteTimeout
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	pending, err := s.mesh.StartTraceRoute(ctx, mesh.TraceRouteOptions{
		To:       uint32(req.To),
		Channel:  req.Channel,
		HopLimit: req.HopLimit,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"requestID": pending.RequestID,
		"to":        pending.To,
		"channel":   pending.Channel,
		"hopLimit":  pending.HopLimit,
		"createdAt": pending.CreatedAt,
		"expiresAt": pending.ExpiresAt,
		"status":    "pending",
	})
}

func (s *apiServer) handleTraceRoutes(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	limit := queryInt(r.URL.Query(), "limit", 200)
	node := uint32(queryInt(r.URL.Query(), "node", 0))
	routes, err := s.mesh.TraceRoutes(r.Context(), node, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

func (s *apiServer) handlePendingTraceRoutes(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.mesh.PendingTraces())
}

func (s *apiServer) handleRadioSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		settings, err := s.mesh.RadioSettings(ctx)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	case http.MethodPost:
		var req radioSettingsUpdateRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		settings, err := s.mesh.UpdateRadioSettings(ctx, mesh.RadioSettingsUpdate{
			HopLimit:  req.HopLimit,
			TxEnabled: req.TxEnabled,
			Role:      req.Role,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *apiServer) handleFixedLocation(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req fixedPositionRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if err := s.mesh.SetFixedPosition(ctx, mesh.FixedPosition{
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
			Altitude:  req.Altitude,
		}); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if err := s.mesh.ClearFixedPosition(ctx); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		methodNotAllowed(w, http.MethodPost, http.MethodDelete)
	}
}

func (s *apiServer) handleSpoofTemperature(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req spoofTemperatureRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.mesh.SendSpoofTemperature(mesh.SpoofTemperatureOptions{
		Channel:     req.Channel,
		Temperature: req.Temperature,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":          id,
		"temperature": req.Temperature,
		"channel":     req.Channel,
	})
}

func (s *apiServer) handleSpoofEnvironment(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req spoofEnvironmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.mesh.SendSpoofEnvironment(mesh.SpoofEnvironmentOptions{
		Channel:     req.Channel,
		Temperature: req.Temperature,
		Humidity:    req.Humidity,
		Pressure:    req.Pressure,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"id": id, "channel": req.Channel})
}

func (s *apiServer) handleSpoofDevice(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req spoofDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.mesh.SendSpoofDevice(mesh.SpoofDeviceOptions{
		Channel:            req.Channel,
		BatteryLevel:       req.BatteryLevel,
		Voltage:            req.Voltage,
		ChannelUtilization: req.ChannelUtilization,
		AirUtilTX:          req.AirUtilTX,
		UptimeSeconds:      req.UptimeSeconds,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"id": id, "channel": req.Channel})
}

func (s *apiServer) handleSpoofPower(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req spoofPowerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.mesh.SendSpoofPower(mesh.SpoofPowerOptions{
		Channel:    req.Channel,
		Ch1Voltage: req.Ch1Voltage,
		Ch1Current: req.Ch1Current,
		Ch2Voltage: req.Ch2Voltage,
		Ch2Current: req.Ch2Current,
		Ch3Voltage: req.Ch3Voltage,
		Ch3Current: req.Ch3Current,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"id": id, "channel": req.Channel})
}

func (s *apiServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming is not supported"))
		return
	}

	messages, unsubscribe := s.mesh.Subscribe(64)
	defer unsubscribe()
	positions, unsubscribePositions := s.mesh.SubscribePositions(64)
	defer unsubscribePositions()
	environments, unsubscribeEnvironments := s.mesh.SubscribeEnvironment(64)
	defer unsubscribeEnvironments()
	devices, unsubscribeDevices := s.mesh.SubscribeDevice(64)
	defer unsubscribeDevices()
	powers, unsubscribePowers := s.mesh.SubscribePower(64)
	defer unsubscribePowers()
	airs, unsubscribeAirs := s.mesh.SubscribeAir(64)
	defer unsubscribeAirs()
	locals, unsubscribeLocals := s.mesh.SubscribeLocalStats(64)
	defer unsubscribeLocals()
	healths, unsubscribeHealths := s.mesh.SubscribeHealth(64)
	defer unsubscribeHealths()
	traces, unsubscribeTraces := s.mesh.SubscribeTraces(64)
	defer unsubscribeTraces()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	channel := r.URL.Query().Get("channel")
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case message, ok := <-messages:
			if !ok {
				return
			}
			if channel != "" && !sameChannel(message.Channel.Name, channel) {
				continue
			}
			if err := writeSSE(w, "message.received", eventEnvelope{
				Type: "message.received",
				Time: time.Now(),
				Data: message,
			}); err != nil {
				return
			}
			flusher.Flush()
		case position, ok := <-positions:
			if !ok {
				return
			}
			if err := writeSSE(w, "position.updated", eventEnvelope{
				Type: "position.updated",
				Time: time.Now(),
				Data: position,
			}); err != nil {
				return
			}
			flusher.Flush()
		case environment, ok := <-environments:
			if !ok {
				return
			}
			if err := writeSSE(w, "environment.updated", eventEnvelope{
				Type: "environment.updated",
				Time: time.Now(),
				Data: environment,
			}); err != nil {
				return
			}
			flusher.Flush()
		case sample, ok := <-devices:
			if !ok {
				return
			}
			if err := writeSSE(w, "device.updated", eventEnvelope{Type: "device.updated", Time: time.Now(), Data: sample}); err != nil {
				return
			}
			flusher.Flush()
		case sample, ok := <-powers:
			if !ok {
				return
			}
			if err := writeSSE(w, "power.updated", eventEnvelope{Type: "power.updated", Time: time.Now(), Data: sample}); err != nil {
				return
			}
			flusher.Flush()
		case sample, ok := <-airs:
			if !ok {
				return
			}
			if err := writeSSE(w, "airquality.updated", eventEnvelope{Type: "airquality.updated", Time: time.Now(), Data: sample}); err != nil {
				return
			}
			flusher.Flush()
		case sample, ok := <-locals:
			if !ok {
				return
			}
			if err := writeSSE(w, "localstats.updated", eventEnvelope{Type: "localstats.updated", Time: time.Now(), Data: sample}); err != nil {
				return
			}
			flusher.Flush()
		case sample, ok := <-healths:
			if !ok {
				return
			}
			if err := writeSSE(w, "health.updated", eventEnvelope{Type: "health.updated", Time: time.Now(), Data: sample}); err != nil {
				return
			}
			flusher.Flush()
		case route, ok := <-traces:
			if !ok {
				return
			}
			if err := writeSSE(w, "trace.updated", eventEnvelope{Type: "trace.updated", Time: time.Now(), Data: route}); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

func writeSSE(w http.ResponseWriter, event string, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	return err
}

func allowMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		methodNotAllowed(w, method)
		return false
	}
	return true
}

func queryInt(values url.Values, key string, fallback int) int {
	raw := strings.TrimSpace(values.Get(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func filterMessages(messages []mesh.Message, channel string) []mesh.Message {
	filtered := make([]mesh.Message, 0, len(messages))
	for _, message := range messages {
		if sameChannel(message.Channel.Name, channel) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func sameChannel(left string, right string) bool {
	return displayChannel(left) == displayChannel(right)
}

func displayChannel(name string) string {
	if name == "" {
		return "Primary"
	}
	return name
}

func parseCORSOrigins(value string) []string {
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func (n *nodeNum) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*n = 0
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			*n = 0
			return nil
		}
		base := 10
		if strings.HasPrefix(value, "!") {
			base = 16
			value = strings.TrimPrefix(value, "!")
		}
		parsed, err := strconv.ParseUint(value, base, 32)
		if err != nil {
			return fmt.Errorf("node number string must be decimal like \"12345678\" or hex like \"!12345678\": %w", err)
		}
		*n = nodeNum(parsed)
		return nil
	}

	parsed, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return fmt.Errorf("node number must be a uint32 number or hex string: %w", err)
	}
	*n = nodeNum(parsed)
	return nil
}
