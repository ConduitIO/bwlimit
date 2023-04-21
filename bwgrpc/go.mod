module github.com/conduitio/bwlimit/bwgrpc

go 1.20

// TODO remove replacement
replace github.com/conduitio/bwlimit => ../.

require (
	github.com/conduitio/bwlimit v0.0.0-00010101000000-000000000000
	github.com/golang/mock v1.6.0
	google.golang.org/grpc v1.54.0
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
)
