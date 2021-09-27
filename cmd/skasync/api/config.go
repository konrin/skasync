package api

type Config struct {
	Port int
}

func DefaultConfig() Config {
	return Config{
		Port: 60001,
	}
}
