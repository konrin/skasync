package skaffold

type Config struct {
	Addr string
}

func DefaultConfig() Config {
	return Config{
		Addr: "127.0.0.1:50052",
	}
}
