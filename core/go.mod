module github.com/kienvan102/monorepo-microservices-template/core

go 1.27.1

require github.com/kienvan102/monorepo-microservices-template/commonlib v0.0.0

require (
	github.com/caarlos0/env/v11 v11.4.1 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

replace github.com/kienvan102/monorepo-microservices-template/commonlib => ../commonlib
