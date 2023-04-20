package proto

//go:generate mockgen -destination=mock.go -package=proto -mock_names=TestServiceServer=MockTestServiceServer github.com/conduitio/conn-rate-limit/proto TestServiceServer
