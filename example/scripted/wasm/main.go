package main

import (
	"context"
	"io/ioutil"
	"reflect"

	"github.com/mathetake/gasm/hostfunc"
	"github.com/mathetake/gasm/wasm"
	"github.com/rs/zerolog/log"
	"github.com/wasmerio/wasmer-go/wasmer"
	// "github.com/mathetake/gasm/hostfunc"
	// "github.com/mathetake/gasm/wasi"
	// "github.com/mathetake/gasm/wasm"
)

func noError(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Fatal().Err(err).Msgf(msg, args...)
	}
}

func main() {
	ctx := context.Background()
	buf, err := ioutil.ReadFile("wasm_programs/ping.wasm")
	noError(err, "Failed to load program")
	// mod, err := wasm.DecodeModule(bytes.NewBuffer(buf))
	// noError(err, "Failed to parse file as a valid WASM binary")

	engine := wasmer.NewUniversalEngine()
	store := wasmer.NewStore(engine)
	defer store.Close()

	noError(err, "Unable to create memory limits")
	module, err := wasmer.NewModule(store, buf)
	noError(err, "Unable to create wasm module")

	// modules := wasi.New().Modules()
	// modules, err = exportStageFunctions(modules)
	// noError(err, "Unable to export stage functions")

	wasiEnv, err := wasmer.NewWasiStateBuilder("ping.wasm").
		Environment("RUNTIME", "WASMER").
		MapDirectory(".", ".").
		Finalize()
	noError(err, "Unable to create wasi environment")

	var instance *wasmer.Instance

	importObject, err := wasiEnv.GenerateImportObject(store, module)
	noError(err, "Unable to create importObject from wasi env")
	importObject.Register("env", exportStageFunctionsWASMER(ctx, &instance, store))

	instance, err = wasmer.NewInstance(module, importObject)
	noError(err, "Unable to create instance of module")

	start, err := instance.Exports.GetWasiStartFunction()
	noError(err, "Unable to get was start function")

	_, err = start()
	noError(err, "Start function failed")

	actorMain, err := instance.Exports.GetFunction("actor_main")
	noError(err, "Unable to obtain actor_main function from module")

	ret, err := actorMain()
	noError(err, "Call to actorMain failed")

	switch ret := ret.(type) {
	case int32:
		switch ret {
		case 0:
			log.Info().Msgf("actor_main exit with 0 code")
		default:
			log.Fatal().Int32("exitCode", ret).Msg("actor_main exit with non-zero code")
		}
	default:
		log.Fatal().Interface("exitValue", ret).Msg("actor_main exit with non int32 value")
	}

	// vm, err := wasm.NewVM(mod, modules)
	// noError(err, "Failed to create Wasm VM with WASI modules")
	// ret, _, err := vm.ExecExportedFunction("actor_main")
	// noError(err, "function actor_main failed with an error")
	// switch len(ret) {
	// case 0:
	// 	log.Info().Msgf("actor_main did not return any values")
	// default:
	// 	if ret[0] != 0 {
	// 		noError(fmt.Errorf("non-zero code: %v", ret), "actor_main returned a non-zero code")
	// 	} else {
	// 		log.Info().Msgf("actor_main exit with 0")
	// 	}
	// }
}

func exportStageFunctionsWASMER(ctx context.Context, instance **wasmer.Instance, store *wasmer.Store) map[string]wasmer.IntoExtern {
	ns := make(map[string]wasmer.IntoExtern)
	ns["stage_log_info"] = wasmer.NewFunctionWithEnvironment(
		store,
		wasmer.NewFunctionType(
			wasmer.NewValueTypes(wasmer.I32, wasmer.I32), // zero argument
			wasmer.NewValueTypes(wasmer.I32),             // one i32 result
		),
		ctx,
		func(environment interface{}, args []wasmer.Value) ([]wasmer.Value, error) {
			println("here")
			mem, err := (*instance).Exports.Get("memory")
			if err != nil {
				return nil, err
			}
			str := string(mem.IntoMemory().Data()[args[0].I32() : args[0].I32()+args[1].I32()])
			_ = str
			log.Info().Interface("memory", mem).Int32("pages", int32(mem.IntoMemory().Size())).Int32("ptr", args[0].I32()).Int32("len", args[0].I32()).Str("msg", str).Send()
			// ctx := environment.(context.Context)
			// _ = ctx

			return []wasmer.Value{wasmer.NewI32(42)}, nil
		},
	)
	return ns
}

func exportStageFunctions(parent map[string]*wasm.Module) (map[string]*wasm.Module, error) {
	b := hostfunc.NewModuleBuilderWith(parent)
	b.MustSetFunction("env", "stage_log_info", func(machine *wasm.VirtualMachine) reflect.Value {
		return reflect.ValueOf(func(bufPtr, bufLen uint32) {
			log.Debug().Int("memorySize", len(machine.Memory)).Send()
			mem := machine.Memory[bufPtr:]
			str := string(mem[:bufLen])
			log.Info().Uint32("bufPtr", bufPtr).Uint32("len", bufLen).Str("body", str).Msg("stage_log_info")
			// str := string(machine.Memory[offset:length])
			// log.Info().Uint32("offset", offset).Uint32("length", length).Str("body", str).Msg("String from guest to host")
			// log.Info().Msg("stage_exports_print_number called")
		})
	})
	return b.Done(), nil
}
