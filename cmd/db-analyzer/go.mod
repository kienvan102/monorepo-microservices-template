module github.com/kienvan102/monorepo-microservices-template/cmd/db-analyzer

go 1.27.1

require (
	github.com/google/wire v0.7.0
	github.com/kienvan102/monorepo-microservices-template/commonlib v0.0.0
	github.com/kienvan102/monorepo-microservices-template/core v0.0.0
	github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer v0.0.0
)

require (
	github.com/caarlos0/env/v11 v11.4.1 // indirect
	github.com/google/subcommands v1.2.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.mongodb.org/mongo-driver/v2 v2.8.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/mod v0.20.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/tools v0.24.1 // indirect
)

replace github.com/kienvan102/monorepo-microservices-template/core => ../../core

replace github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer => ../../services/mongoanalyzer

tool github.com/google/wire/cmd/wire

replace github.com/kienvan102/monorepo-microservices-template/commonlib => ../../commonlib
