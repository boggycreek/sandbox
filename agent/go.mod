module github.com/boggycreek/sandbox/agent

go 1.27.1

require (
	github.com/boggycreek/sandbox/backplane v0.0.0
	github.com/boggycreek/sandbox/mcp v0.0.0
)

replace (
	github.com/boggycreek/sandbox/backplane => ../backplane
	github.com/boggycreek/sandbox/mcp => ../mcp
)
