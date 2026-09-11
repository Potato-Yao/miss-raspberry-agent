package napcat

import "time"

// defaultDirectoryRefreshInterval is used when a client config leaves
// DirectoryRefreshInterval unset.
const defaultDirectoryRefreshInterval = 30 * time.Minute

type NapcatClientConfig struct {
	WebSocketURL  string
	AccessToken   string
	NickName      []string
	CommandPrefix string
	SuperUsers    []int64

	// DirectoryRefreshInterval controls how often the in-memory group member
	// directory is refreshed after the initial load. Zero means the default.
	DirectoryRefreshInterval time.Duration
}
