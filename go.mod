module github.com/andrebq/stage

go 1.16

replace github.com/andrebq/jtb => ../jtb

require (
	github.com/andrebq/jtb v0.0.0-20210821191106-61403c72e392
	github.com/dop251/goja v0.0.0-20210802212625-c04c64313062
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/rs/zerolog v1.23.0
	github.com/urfave/cli/v2 v2.3.0
	github.com/valyala/fastjson v1.6.3
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
)
