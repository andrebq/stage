package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/andrebq/jtb/engine"
	"github.com/andrebq/stage/client"
	"github.com/andrebq/stage/internal/protocol"
	"github.com/dop251/goja"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fastjson"
)

type (
	stageModule struct {
		actorID string
		bridge  *client.WSBridge
		ctx     context.Context
	}
)

var (
	bridgeAddr = flag.String("bridge", fmt.Sprintf("localhost:%v", protocol.DefaultBridgePort), "Address of the Websocket bridge")
	actorID    = flag.String("actorID", "", "ID of the actor")
	codeFile   = flag.String("code", "index.js", "File to use as starting point")
	anchorCode = flag.Bool("anchor", true, "Anchor the scripts to the directory holding the code file")
)

func main() {
	flag.Parse()
	if *actorID == "" {
		panic(errors.New("actor id is required"))
	}

	var code []byte
	var err error
	code, err = ioutil.ReadFile(*codeFile)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	e, err := engine.New()
	e.ConnectStdio(os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		panic(err)
	}
	if *anchorCode {
		err = e.AnchorModules(filepath.Dir(*codeFile))
		if err != nil {
			panic(fmt.Errorf("unable to anchor modules at %v, cause %w", filepath.Dir(*codeFile), err))
		}
	}
	wsURL := fmt.Sprintf("ws://%v?actorid=%v", *bridgeAddr, actorID)
	stageModule, err := NewStageModule(ctx, wsURL, log.Logger)
	if err != nil {
		panic(err)
	}
	err = e.AddBuiltin("@stage", false, stageModule)
	if err != nil {
		panic(err)
	}
	_, err = e.InteractiveEval(string(code))
	if err != nil {
		panic(err)
	}
}

func NewStageModule(ctx context.Context, bridgeAddr string, logger zerolog.Logger) (*stageModule, error) {
	b, err := client.NewBridge(ctx, bridgeAddr, logger)
	if err != nil {
		return nil, err
	}
	return &stageModule{
		bridge: b,
		ctx:    ctx,
	}, nil
}

func (sm *stageModule) DefineModule(exports *goja.Object, runtime *goja.Runtime) error {
	exports.Set("read", sm.readMessage(runtime))
	exports.Set("write", sm.writeMessage(runtime))
	return nil
}

func (sm *stageModule) readMessage(runtime *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(fc goja.FunctionCall) goja.Value {
		select {
		case <-sm.ctx.Done():
			panic(runtime.NewGoError(errors.New("connection to bridge is closed")))
		case msg := <-sm.bridge.Input():
			return runtime.ToValue(string(msg))
		}
	}
}

func (sm *stageModule) writeMessage(runtime *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(fc goja.FunctionCall) goja.Value {
		value := fc.Argument(0).ToString().Export().(string)
		if err := fastjson.Validate(value); err != nil {
			panic(runtime.NewGoError(err))
		}
		select {
		case <-sm.ctx.Done():
			panic(runtime.NewGoError(errors.New("connection to bridge is closed")))
		case sm.bridge.Output() <- json.RawMessage([]byte(value)):
			return runtime.ToValue(true)
		}
	}
}
