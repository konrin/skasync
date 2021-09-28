package skaffold

type Config struct {
	Addr                 string
	WatchingDeployStatus bool
}

func DefaultConfig() Config {
	return Config{
		Addr:                 "127.0.0.1:50052",
		WatchingDeployStatus: true,
	}
}
