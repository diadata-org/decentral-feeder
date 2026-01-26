module github.com/diadata-org/decentral-feeder

go 1.23

// toolchain go1.22.3

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

require github.com/google/go-github/v56 v56.0.0

require golang.org/x/sys v0.13.0 // indirect

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/sirupsen/logrus v1.9.3
	golang.org/x/net v0.16.0 // indirect
	golang.org/x/oauth2 v0.13.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
