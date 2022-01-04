package config

type Connect struct {
	Keepalive      int `toml:"keepalive"  validate:"default=100"`
	WriteTimeout   int `toml:"writeTimeout"  validate:"default=50"`
	ConnectTimeout int `toml:"connectTimeout" validate:"default=1000"`
	AckTimeout     int `toml:"ackTimeout" validate:"default=5000"`
	TimeoutRetries int `toml:"timeOutRetries" validate:"default=2"`
}

type Provider struct {
	SessionsProvider string `toml:"sessionsProvider"`
	TopicsProvider   string `toml:"topicsProvider"`
	Authenticator    string `toml:"authenticator"`
}

type DefaultConfig struct {
	Connect  `toml:"connect"`
	Provider `toml:"provider"`
	Auth     `toml:"auth"`
	Server   `toml:"server"`
}

type Auth struct {
	Allows []string `toml:"allows"`
}

type Server struct {
	Redirects         []string `tome:"redirects"`
	RedirectOpen      bool     `tome:"redirectOpen"`
	RedirectIsForEver bool     `tome:"redirectIsForEver"`
}
