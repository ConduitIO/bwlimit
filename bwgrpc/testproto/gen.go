package testproto

//go:generate mockgen -destination=mock.go -package=testproto -mock_names=TestServiceServer=MockTestServiceServer github.com/conduitio/bwlimit/bwgrpc/testproto TestServiceServer
