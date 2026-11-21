package config

type Config struct {
	Listen    string `json:"listen"`
	PGDSN     string `json:"pg_dsn"`
	ProxyAddr string `json:"proxy_addr,omitempty"`
	DataDir   string `json:"data_dir"`
	SkillDir  string `json:"skill_dir,omitempty"`
}

func Default() Config {
	return Config{
		Listen:  "localhost:9400",
		DataDir: ".",
	}
}
