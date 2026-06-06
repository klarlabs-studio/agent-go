module go.klarlabs.de/agent/contrib/pack-jira

go 1.25.0

require (
	github.com/felixgeelhaar/jirasdk v1.0.0
	go.klarlabs.de/agent v0.0.0
)

require golang.org/x/oauth2 v0.36.0 // indirect

replace go.klarlabs.de/agent => ../..
