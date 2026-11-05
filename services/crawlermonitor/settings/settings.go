package settings

import "time"

type Config struct {
	Interval time.Duration `env:"INTERVAL" envDefault:"5m"`

	Mongo MongoConfig `envPrefix:"MONGO_"`
	MySQL MySQLConfig `envPrefix:"MYSQL_"`
}

type MongoConfig struct {
	URI          string        `env:"MONGO_URI" envDefault:"mongodb://localhost:27017"`
	Database     string        `env:"MONGO_DATABASE" envDefault:"ynm_crawler"`
	QueryTimeout time.Duration `env:"MONGO_QUERY_TIMEOUT" envDefault:"1m"`
	PingTimeout  time.Duration `env:"MONGO_PING_TIMEOUT" envDefault:"12s"`
	ServerSelectionTimeout time.Duration `env:"MONGO_SERVER_SELECTION_TIMEOUT" envDefault:"30s"`
}

type MySQLConfig struct {
	Enabled  bool   `env:"ENABLED" envDefault:"false"`
	Host     string `env:"HOST"`
	Port     int    `env:"PORT" envDefault:"3306"`
	Database string `env:"DATABASE"`
	Username string `env:"USERNAME"`
	Password string `env:"PASSWORD"`
}