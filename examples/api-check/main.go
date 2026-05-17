package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"meshin/mesh"
)

type rxMsg mesh.Message
type closedMsg struct{}
type sentMsg struct {
	id   uint32
	text string
}
type sendErrMsg struct {
	err error
}
type channelOpMsg struct {
	status   string
	err      error
	channels []mesh.Channel
}
type traceMsg struct {
	status string
	err    error
	route  mesh.TraceRoute
}
type nodesMsg struct {
	nodes []mesh.Node
	err   error
}

type uiMode int

const (
	modeChat uiMode = iota
	modeMainMenu
	modeChannelMenu
	modeAddChannel
	modeNodes
	modeChannels
	modeTools
	modeTraceTarget
)

type model struct {
	api         *apiClient
	rx          <-chan mesh.Message
	input       textinput.Model
	history     []mesh.Message
	channels    []mesh.Channel
	nodes       []mesh.Node
	selected    int
	mode        uiMode
	menu        int
	channelMenu int
	node        int
	tracePick   int
	trace       *mesh.TraceRoute
	status      string
	width       int
	height      int
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Padding(0, 1)
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func main() {
	apiURL := flag.String("api", "http://127.0.0.1:8080", "gomeshind API base URL")
	flag.Parse()

	client, err := newAPIClient(*apiURL)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages := client.Events(ctx)

	loadCtx, loadCancel := context.WithTimeout(ctx, 10*time.Second)
	defer loadCancel()

	program := tea.NewProgram(newModel(loadCtx, client, messages, *apiURL), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}

func newModel(ctx context.Context, api *apiClient, messages <-chan mesh.Message, apiURL string) model {
	input := textinput.New()
	input.Placeholder = "message"
	input.Prompt = "> "
	input.Focus()

	channels, err := api.Channels(ctx)
	if err != nil || len(channels) == 0 {
		channels = []mesh.Channel{{Index: 0, Name: "Primary", Role: "PRIMARY"}}
	}
	channels = activeChannels(channels)
	nodes, _ := api.Nodes(ctx)
	history, _ := api.Messages(ctx)

	return model{
		api:      api,
		rx:       messages,
		input:    input,
		history:  history,
		channels: channels,
		nodes:    nodes,
		status:   fmt.Sprintf("connected api=%s", apiURL),
	}
}

func (m model) Init() tea.Cmd {
	return waitForRx(m.rx)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(20, msg.Width-4)
	case tea.KeyMsg:
		handled := true
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			switch m.mode {
			case modeTraceTarget:
				m.mode = modeTools
				m.input.SetValue("")
				m.input.Placeholder = "message"
				m.status = "tools"
			case modeAddChannel:
				m.mode = modeChannelMenu
				m.input.SetValue("")
				m.input.Placeholder = "message"
				m.status = "channel menu"
			case modeMainMenu, modeChannelMenu, modeNodes, modeChannels, modeTools:
				m.mode = modeChat
				m.input.Placeholder = "message"
				m.input.SetValue("")
				m.status = "connected"
			default:
				m.mode = modeMainMenu
				m.status = "menu"
			}
		case "tab":
			if m.mode == modeChat {
				m.nextChannel()
			}
		case "shift+tab":
			if m.mode == modeChat {
				m.previousChannel()
			}
		case "up":
			m.moveSelection(-1)
		case "down":
			m.moveSelection(1)
		case "enter":
			text := strings.TrimSpace(m.input.Value())
			switch m.mode {
			case modeChat:
				if text == "" {
					break
				}
				channel := m.currentChannel()
				m.input.SetValue("")
				m.status = "sending"
				cmds = append(cmds, sendText(m.api, channel.Name, text))
			case modeMainMenu:
				switch m.menu {
				case 0:
					m.mode = modeChat
					m.status = "connected"
				case 1:
					m.mode = modeNodes
					m.status = "nodes"
					cmds = append(cmds, loadNodes(m.api))
				case 2:
					m.mode = modeChannels
					m.status = "channels"
				case 3:
					m.mode = modeTools
					m.status = "tools"
				case 4:
					m.mode = modeChannelMenu
					m.status = "channel menu"
				case 5:
					return m, tea.Quit
				}
			case modeChannelMenu:
				if m.channelMenu == 0 {
					m.mode = modeAddChannel
					m.input.SetValue("")
					m.input.Placeholder = "new channel name"
					m.status = "enter channel name"
				} else {
					channel := m.currentChannel()
					m.mode = modeChat
					m.status = "removing channel"
					cmds = append(cmds, removeChannel(m.api, channel.Name))
				}
			case modeAddChannel:
				if text == "" {
					break
				}
				m.input.SetValue("")
				m.input.Placeholder = "message"
				m.mode = modeChat
				m.status = "adding channel"
				cmds = append(cmds, addChannel(m.api, text))
			case modeTools:
				m.mode = modeTraceTarget
				m.tracePick = 0
				m.input.SetValue("")
				m.input.Placeholder = "search node"
				m.status = "select traceroute target"
				cmds = append(cmds, loadNodes(m.api))
			case modeTraceTarget:
				node := m.currentTraceNode()
				if node.Num == 0 {
					m.status = "no node selected"
					break
				}
				m.node = m.nodeIndex(node.Num)
				m.mode = modeTools
				m.input.SetValue("")
				m.input.Placeholder = "message"
				m.status = "sending traceroute"
				cmds = append(cmds, traceRoute(m.api, node.Num, m.currentChannel().Name))
			}
		default:
			handled = m.mode != modeChat && m.mode != modeAddChannel && m.mode != modeTraceTarget
		}
		if handled {
			return m, tea.Batch(cmds...)
		}
	case rxMsg:
		m.history = append(m.history, mesh.Message(msg))
		if len(m.history) > 300 {
			m.history = m.history[len(m.history)-300:]
		}
		cmds = append(cmds, waitForRx(m.rx))
	case closedMsg:
		m.status = "radio stream closed"
	case sentMsg:
		m.status = fmt.Sprintf("sent %08x", msg.id)
	case sendErrMsg:
		m.status = "send failed: " + msg.err.Error()
	case channelOpMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = msg.status
			m.channels = activeChannels(msg.channels)
			if m.selected >= len(m.channels) {
				m.selected = max(0, len(m.channels)-1)
			}
		}
	case traceMsg:
		if msg.err != nil {
			m.status = "traceroute failed: " + msg.err.Error()
		} else {
			m.status = msg.status
			m.trace = &msg.route
		}
	case nodesMsg:
		if msg.err != nil {
			m.status = "nodes failed: " + msg.err.Error()
		} else {
			m.nodes = msg.nodes
			if m.node >= len(m.nodes) {
				m.node = max(0, len(m.nodes)-1)
			}
			filtered := m.filteredTraceNodes()
			if m.tracePick >= len(filtered) {
				m.tracePick = max(0, len(filtered)-1)
			}
			m.status = fmt.Sprintf("nodes: %d", len(m.nodes))
		}
	}

	var cmd tea.Cmd
	previousInput := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.mode == modeTraceTarget {
		if m.input.Value() != previousInput {
			m.tracePick = 0
		}
		filtered := m.filteredTraceNodes()
		if m.tracePick >= len(filtered) {
			m.tracePick = max(0, len(filtered)-1)
		}
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	width := m.width
	if width == 0 {
		width = 80
	}
	height := m.height
	if height == 0 {
		height = 24
	}

	header := headerStyle.Width(width).Render("GoMeshin API Check")
	channel := accentStyle.Render("channel: "+displayChannel(m.currentChannel().Name)) +
		mutedStyle.Render("  enter send  tab switch channel  esc menu")
	body := renderBody(m, max(4, height-7), width)
	status := mutedStyle.Render(m.status)
	if strings.Contains(m.status, "failed") || strings.Contains(m.status, "closed") {
		status = errorStyle.Render(m.status)
	}

	return strings.Join([]string{
		header,
		channel,
		body,
		status,
		m.input.View(),
	}, "\n")
}

func renderBody(m model, height int, width int) string {
	switch m.mode {
	case modeMainMenu:
		return renderMainMenu(m, height)
	case modeChannelMenu:
		return renderChannelMenu(m, height)
	case modeNodes:
		return renderNodes(m, height)
	case modeChannels:
		return renderChannels(m, height)
	case modeTools:
		return renderTools(m, height)
	case modeTraceTarget:
		return renderTraceTarget(m, height, width)
	default:
		return renderMessages(m.history, m.currentChannel().Name, height, width)
	}
}

func renderMainMenu(m model, height int) string {
	items := mainMenuItems()
	lines := []string{accentStyle.Render("Menu")}
	for index, item := range items {
		prefix := "  "
		if index == m.menu {
			prefix = "> "
		}
		lines = append(lines, prefix+item)
	}
	lines = append(lines, "", mutedStyle.Render("enter select  up/down move  esc close menu"))

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func renderChannelMenu(m model, height int) string {
	items := channelMenuItems()
	lines := []string{accentStyle.Render("Channel menu")}
	for index, item := range items {
		prefix := "  "
		if index == m.channelMenu {
			prefix = "> "
		}
		lines = append(lines, prefix+item)
	}
	lines = append(lines, "", mutedStyle.Render("enter select  up/down move  esc close menu"))

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func mainMenuItems() []string {
	return []string{"Messages", "Nodes", "Channels", "Tools", "Edit channels", "Exit"}
}

func channelMenuItems() []string {
	return []string{"Add channel", "Remove selected channel"}
}

func renderNodes(m model, height int) string {
	lines := []string{accentStyle.Render("Nodes")}
	if len(m.nodes) == 0 {
		lines = append(lines, "No nodes known yet.")
	} else {
		for index, node := range m.nodes {
			prefix := "  "
			if index == m.node {
				prefix = "> "
			}
			label := node.ShortName
			if label == "" {
				label = node.LongName
			}
			lines = append(lines, fmt.Sprintf("%s!%08x  %-8s  %s", prefix, node.Num, label, node.LongName))
		}
	}
	lines = append(lines, "", mutedStyle.Render("up/down select  esc close"))
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func renderChannels(m model, height int) string {
	lines := []string{accentStyle.Render("Channels")}
	for _, channel := range m.channels {
		lines = append(lines, fmt.Sprintf("  [%d] %-10s %s", channel.Index, channel.Role, displayChannel(channel.Name)))
	}
	lines = append(lines, "", mutedStyle.Render("m edit channels  c close"))
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func renderTools(m model, height int) string {
	node := m.currentNode()
	target := "no node selected"
	if node.Num != 0 {
		target = formatNode(node)
	}
	lines := []string{
		accentStyle.Render("Tools"),
		"Traceroute",
		"  target: " + target,
		"",
		mutedStyle.Render("enter select/search target  esc close"),
	}
	if m.trace != nil {
		lines = append(lines, "", accentStyle.Render("Last traceroute"))
		lines = append(lines, "  towards: "+formatTraceHops(m.trace.Towards))
		if len(m.trace.Back) > 0 {
			lines = append(lines, "  back:    "+formatTraceHops(m.trace.Back))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func renderTraceTarget(m model, height int, width int) string {
	filtered := m.filteredTraceNodes()
	lines := []string{
		accentStyle.Render("Traceroute target"),
		mutedStyle.Render("type to search  up/down move  enter run  esc back"),
		"",
	}

	if len(filtered) == 0 {
		lines = append(lines, "No matching nodes.")
	} else {
		available := max(1, height-len(lines))
		start := 0
		if m.tracePick >= available {
			start = m.tracePick - available + 1
		}
		end := min(len(filtered), start+available)
		for index := start; index < end; index++ {
			item := filtered[index]
			prefix := "  "
			if index == m.tracePick {
				prefix = "> "
			}
			line := prefix + formatNode(item.node)
			if len(line) > width && width > 3 {
				line = line[:width-3] + "..."
			}
			lines = append(lines, line)
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m *model) moveSelection(delta int) {
	switch m.mode {
	case modeMainMenu:
		m.menu = wrapIndex(m.menu+delta, len(mainMenuItems()))
	case modeChannelMenu:
		m.channelMenu = wrapIndex(m.channelMenu+delta, len(channelMenuItems()))
	case modeNodes:
		m.node = wrapIndex(m.node+delta, len(m.nodes))
	case modeTraceTarget:
		m.tracePick = wrapIndex(m.tracePick+delta, len(m.filteredTraceNodes()))
	}
}

func (m *model) nextChannel() {
	if len(m.channels) == 0 {
		return
	}
	m.selected = (m.selected + 1) % len(m.channels)
}

func (m *model) previousChannel() {
	if len(m.channels) == 0 {
		return
	}
	m.selected--
	if m.selected < 0 {
		m.selected = len(m.channels) - 1
	}
}

func (m model) currentChannel() mesh.Channel {
	if len(m.channels) == 0 || m.selected < 0 || m.selected >= len(m.channels) {
		return mesh.Channel{Index: 0, Name: "Primary", Role: "PRIMARY"}
	}
	return m.channels[m.selected]
}

func (m *model) nextNode() {
	if len(m.nodes) == 0 {
		return
	}
	m.node = (m.node + 1) % len(m.nodes)
}

func (m *model) previousNode() {
	if len(m.nodes) == 0 {
		return
	}
	m.node--
	if m.node < 0 {
		m.node = len(m.nodes) - 1
	}
}

func (m model) currentNode() mesh.Node {
	if len(m.nodes) == 0 || m.node < 0 || m.node >= len(m.nodes) {
		return mesh.Node{}
	}
	return m.nodes[m.node]
}

type traceNodeItem struct {
	node mesh.Node
}

func (m model) filteredTraceNodes() []traceNodeItem {
	query := strings.ToLower(strings.TrimSpace(m.input.Value()))
	items := make([]traceNodeItem, 0, len(m.nodes))
	for _, node := range m.nodes {
		if node.Num == 0 {
			continue
		}
		if query == "" || nodeMatches(node, query) {
			items = append(items, traceNodeItem{node: node})
		}
	}
	return items
}

func (m model) currentTraceNode() mesh.Node {
	filtered := m.filteredTraceNodes()
	if len(filtered) == 0 || m.tracePick < 0 || m.tracePick >= len(filtered) {
		return mesh.Node{}
	}
	return filtered[m.tracePick].node
}

func (m model) nodeIndex(num uint32) int {
	for index, node := range m.nodes {
		if node.Num == num {
			return index
		}
	}
	return m.node
}

func nodeMatches(node mesh.Node, query string) bool {
	return strings.Contains(strings.ToLower(node.ShortName), query) ||
		strings.Contains(strings.ToLower(node.LongName), query) ||
		strings.Contains(strings.ToLower(fmt.Sprintf("!%08x", node.Num)), query) ||
		strings.Contains(strings.ToLower(fmt.Sprintf("%08x", node.Num)), query)
}

func formatNode(node mesh.Node) string {
	name := node.ShortName
	if name == "" {
		name = node.LongName
	}
	if name == "" {
		name = "(unnamed)"
	}
	if node.LongName != "" && node.LongName != name {
		return fmt.Sprintf("!%08x  %-8s  %s%s", node.Num, name, node.LongName, formatPositionSuffix(node.Position))
	}
	return fmt.Sprintf("!%08x  %s%s", node.Num, name, formatPositionSuffix(node.Position))
}

func formatPositionSuffix(position *mesh.Position) string {
	if position == nil {
		return ""
	}
	return fmt.Sprintf("  %.6f,%.6f", position.Latitude, position.Longitude)
}

func renderMessages(messages []mesh.Message, channel string, height int, width int) string {
	filtered := make([]mesh.Message, 0, len(messages))
	for _, message := range messages {
		if displayChannel(message.Channel.Name) == displayChannel(channel) {
			filtered = append(filtered, message)
		}
	}

	if len(filtered) == 0 {
		return mutedStyle.Height(height).Width(width).Render("No messages yet.")
	}

	start := 0
	if len(filtered) > height {
		start = len(filtered) - height
	}

	lines := make([]string, 0, height)
	for _, message := range filtered[start:] {
		from := fmt.Sprintf("!%08x", message.From.Num)
		if message.From.ShortName != "" {
			from = message.From.ShortName
		}

		line := fmt.Sprintf("[%s] %s: %s", displayChannel(message.Channel.Name), from, message.Text)
		if len(line) > width && width > 3 {
			line = line[:width-3] + "..."
		}
		lines = append(lines, line)
	}

	for len(lines) < height {
		lines = append([]string{""}, lines...)
	}

	return strings.Join(lines, "\n")
}

func waitForRx(messages <-chan mesh.Message) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-messages
		if !ok {
			return closedMsg{}
		}
		return rxMsg(message)
	}
}

func sendText(api *apiClient, channel string, text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		id, err := api.Send(ctx, text, mesh.SendOptions{Channel: channel})
		if err != nil {
			return sendErrMsg{err: err}
		}
		return sentMsg{id: id, text: text}
	}
}

func addChannel(api *apiClient, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		channels, err := api.AddChannel(ctx, name)
		if err != nil {
			return channelOpMsg{err: err}
		}
		return channelOpMsg{status: "channel added: " + name, channels: channels}
	}
}

func removeChannel(api *apiClient, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		channels, err := api.RemoveChannel(ctx, name)
		if err != nil {
			return channelOpMsg{err: err}
		}
		return channelOpMsg{status: "channel removed: " + displayChannel(name), channels: channels}
	}
}

func traceRoute(api *apiClient, target uint32, channel string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 95*time.Second)
		defer cancel()

		route, err := api.TraceRoute(ctx, mesh.TraceRouteOptions{
			To:       target,
			Channel:  channel,
			HopLimit: 3,
		})
		if err != nil {
			return traceMsg{err: err}
		}
		return traceMsg{status: fmt.Sprintf("traceroute %08x complete", route.RequestID), route: route}
	}
}

func loadNodes(api *apiClient) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		nodes, err := api.Nodes(ctx)
		return nodesMsg{nodes: nodes, err: err}
	}
}

func formatTraceHops(hops []mesh.TraceHop) string {
	if len(hops) == 0 {
		return "(empty)"
	}

	parts := make([]string, 0, len(hops))
	for _, hop := range hops {
		label := hop.Node.ShortName
		if label == "" {
			label = fmt.Sprintf("!%08x", hop.Node.Num)
		}
		if hop.SNR != nil {
			label = fmt.Sprintf("%s %.1fdB", label, *hop.SNR)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " -> ")
}

func displayChannel(name string) string {
	if name == "" {
		return "Primary"
	}
	return name
}

func activeChannels(channels []mesh.Channel) []mesh.Channel {
	active := make([]mesh.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel.Role == "DISABLED" {
			continue
		}
		active = append(active, channel)
	}
	if len(active) == 0 {
		return []mesh.Channel{{Index: 0, Name: "Primary", Role: "PRIMARY"}}
	}
	return active
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func wrapIndex(index int, length int) int {
	if length <= 0 {
		return 0
	}
	for index < 0 {
		index += length
	}
	return index % length
}
