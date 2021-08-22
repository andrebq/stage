package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/andrebq/jtb/engine"
	"github.com/andrebq/stage/client"
	"github.com/andrebq/stage/internal/protocol"
	"github.com/dop251/goja"
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

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

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
	wsURL := fmt.Sprintf("ws://%v?actorid=%v", *bridgeAddr, *actorID)
	bridge, err := client.NewBridge(ctx, wsURL, log.Logger)
	if err != nil {
		panic(err)
	}
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	go func() {
		bridge.Run(ctx)
		// abort the actor as soon as the bridge is disconnected
		// without a chance of being restored
		cancel()
	}()
	actorModule, err := NewActorModule(ctx, *actorID, bridge)
	if err != nil {
		panic(err)
	}
	err = e.AddBuiltin("@actor", false, actorModule)
	if err != nil {
		panic(err)
	}
	_, err = e.InteractiveEval(string(code))
	if err != nil {
		panic(err)
	}
}

func NewActorModule(ctx context.Context, actorID string, bridge *client.WSBridge) (*stageModule, error) {
	return &stageModule{
		bridge:  bridge,
		ctx:     ctx,
		actorID: actorID,
	}, nil
}

func (sm *stageModule) DefineModule(exports *goja.Object, runtime *goja.Runtime) error {
	exports.Set("id", sm.id(runtime))
	exports.Set("read", sm.readMessage(runtime))
	exports.Set("write", sm.writeMessage(runtime))
	return nil
}

func (sm *stageModule) id(runtime *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(fc goja.FunctionCall) goja.Value {
		return runtime.ToValue(sm.actorID)
	}
}

func (sm *stageModule) readMessage(runtime *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(fc goja.FunctionCall) goja.Value {
		select {
		case <-sm.ctx.Done():
			panic(runtime.NewGoError(errors.New("connection to bridge is closed")))
		case rawBytes := <-sm.bridge.Input():
			var msg protocol.WebsocketMessage
			err := json.Unmarshal(rawBytes, &msg)
			if err != nil {
				panic(runtime.NewGoError(err))
			}
			obj := runtime.CreateObject(nil)
			obj.DefineDataProperty("sender", runtime.ToValue(msg.Sender), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
			obj.DefineDataProperty("payload", runtime.ToValue(string(msg.Payload)), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
			// TODO: think about how to freeze the object directly from Go
			return obj
		}
	}
}

func (sm *stageModule) writeMessage(runtime *goja.Runtime) func(goja.FunctionCall) goja.Value {
	return func(fc goja.FunctionCall) goja.Value {
		destination := fc.Argument(0).ToString().Export().(string)
		payload := fc.Argument(1).ToString().Export().(string)
		if err := fastjson.Validate(payload); err != nil {
			panic(runtime.NewGoError(err))
		}
		msg, err := json.Marshal(protocol.WebsocketMessage{
			Sender:  sm.actorID,
			Payload: json.RawMessage(payload),
			Actor:   destination,
		})
		if err != nil {
			panic(runtime.NewGoError(err))
		}
		select {
		case <-sm.ctx.Done():
			panic(runtime.NewGoError(errors.New("connection to bridge is closed")))
		case sm.bridge.Output() <- json.RawMessage(msg):
			return runtime.ToValue(true)
		}
	}
}
