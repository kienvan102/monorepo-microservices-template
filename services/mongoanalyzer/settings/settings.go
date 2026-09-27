// Package settings holds mongoanalyzer's configuration. It is its own package
// so mongoanalyzer, mongodb and jsrunner can all import it without a cycle.
// The service declares what it needs; loading it is the deployment's job.
// Process-wide settings such as APP_ENV are not declared here: the process
// framework reads them and passes them down (see core/app.Runtime).
package settings

import (
	"time"

	"github.com/kienvan102/monorepo-microservices-template/core/config"
)

type Config struct {
	Mongo MongoConfig
}

type MongoConfig struct {
	URI           string        `env:"MONGO_URI" envDefault:"mongodb://localhost:27017"`
	Database      string        `env:"MONGO_DATABASE" envDefault:"test"`
	Collection    string        `env:"MONGO_COLLECTION" envDefault:"threads_posts"`
	OutputDir     config.Path   `env:"OUTPUT_DIR" envDefault:"output"`
	QueryTimeout  time.Duration `env:"QUERY_TIMEOUT" envDefault:"20s"`
	ScriptTimeout time.Duration `env:"SCRIPT_TIMEOUT" envDefault:"3m"`
	SampleSize    int           `env:"SAMPLE_SIZE" envDefault:"500"`
	ExplainLimit  int           `env:"EXPLAIN_LIMIT" envDefault:"100"`
}
