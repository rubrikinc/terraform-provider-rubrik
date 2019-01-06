package rubrikcdm

import "github.com/rubrikinc/rubrik-sdk-for-go/rubrikcdm"

// Config is per-provider, specifies where to connect to gitlab
type Config struct {
	NodeIP   string
	Username string
	Password string
}

// Client returns a *gitlab.Client to interact with the configured gitlab instance
func (c *Config) Client() (*rubrikcdm.Credentials, error) {

	rubrik := rubrikcdm.Connect(c.NodeIP, c.Username, c.Password)

	return rubrik, nil
}
