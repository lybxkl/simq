package config

type PProf struct {
	Open bool  `toml:"open" `
	Port int64 `toml:"port" validate:"default=6060"`
}
