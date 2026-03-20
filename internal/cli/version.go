package cli

// Set via ldflags at build time:
//   go build -ldflags "-X github.com/sakakibara/hive/internal/cli.version=1.0.0"
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)
