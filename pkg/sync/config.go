package sync

type Config struct {
	AfterDeployOrStart []string
	Debounce           int
}

func DefaultConfig() Config {
	return Config{
		AfterDeployOrStart: []string{},
		Debounce:           1000,
	}
}
