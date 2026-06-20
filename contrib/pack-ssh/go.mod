module go.klarlabs.de/agent/contrib/pack-ssh

go 1.26.2

require (
	go.klarlabs.de/agent v0.0.0
	golang.org/x/crypto v0.51.0
)

require golang.org/x/sys v0.45.0 // indirect

replace go.klarlabs.de/agent => ../..
