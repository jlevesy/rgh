package memberring

import "github.com/hashicorp/memberlist"

type Config struct {
	Role Role
	List memberlist.Config
}

func NewDefaultLocalConfig(r Role) *Config {
	return &Config{
		Role: r,
		List: *memberlist.DefaultLocalConfig(),
	}
}
