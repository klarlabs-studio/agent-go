module go.klarlabs.de/agent/contrib/storage-redis

go 1.26.2

require (
	github.com/alicebob/miniredis/v2 v2.37.0
	github.com/redis/go-redis/v9 v9.17.2
	go.klarlabs.de/agent v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
)

replace go.klarlabs.de/agent => ../..
