// ============================================
// 共享配置定义
// ============================================
package server

// Config 主配置
type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
		PoolSize int    `yaml:"pool_size"`
	} `yaml:"redis"`
	// OpenObserve struct {
	// 	URL      string `yaml:"url"`
	// 	Org      string `yaml:"org"`
	// 	Email    string `yaml:"email"`
	// 	Password string `yaml:"password"`
	// } `yaml:"openobserve"`
	// Kafka struct {
	// 	Brokers []string   `yaml:"brokers"`
	// 	GroupID string     `yaml:"group_id"`
	// 	Topics  []string   `yaml:"topics"`
	// 	SASL    SASLConfig `yaml:"sasl"`
	// } `yaml:"kafka"`
	Database DatabaseConfig `yaml:"database"`
}
