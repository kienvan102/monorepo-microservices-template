module github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor

go 1.27.1

require (
	github.com/kienvan102/monorepo-microservices-template/commonlib v0.0.0
	github.com/kienvan102/monorepo-microservices-template/core v0.0.0-00010101000000-000000000000
	github.com/prometheus/client_golang v1.25.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/caarlos0/env/v11 v11.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.48.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/kienvan102/monorepo-microservices-template/core => ../../core

replace github.com/kienvan102/monorepo-microservices-template/commonlib => ../../commonlib
