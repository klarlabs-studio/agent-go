module go.klarlabs.de/agent/contrib/pack-websocket

go 1.25.0

require (
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
	go.klarlabs.de/agent v0.0.0
)

require golang.org/x/net v0.54.0 // indirect

replace go.klarlabs.de/agent => ../..
