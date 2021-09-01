package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/rs/zerolog/log"

	"github.com/mathetake/gasm/hostfunc"
	"github.com/mathetake/gasm/wasi"
	"github.com/mathetake/gasm/wasm"
)

func noError(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Fatal().Err(err).Msgf(msg, args...)
	}
}

func main() {
	buf, err := ioutil.ReadFile("wasm_programs/ping.wasm")
	noError(err, "Failed to load program")
	mod, err := wasm.DecodeModule(bytes.NewBuffer(buf))
	noError(err, "Failed to parse file as a valid WASM binary")

	modules := wasi.New().Modules()
	err = exportStageFunctions(modules)
	noError(err, "Unable to export stage functions")

	vm, err := wasm.NewVM(mod, modules)
	noError(err, "Failed to create Wasm VM with WASI modules")
	ret, _, err := vm.ExecExportedFunction("actor_main")
	noError(err, "function actor_main failed with an error")
	switch len(ret) {
	case 0:
		log.Info().Msgf("actor_main did not return any values")
	default:
		if ret[0] != 0 {
			noError(fmt.Errorf("non-zero code: %v", ret), "actor_main returned a non-zero code")
		} else {
			log.Info().Msgf("actor_main exit with 0")
		}
	}
}

func exportStageFunctions(m map[string]*wasm.Module) error {
	b := hostfunc.NewModuleBuilder()
	var err error
	err = b.SetFunction("env", "stage_exports_print_number", func(machine *wasm.VirtualMachine) reflect.Value {
		log.Info().Msg("stage_exports_print_number called")
		return reflect.ValueOf(0)
	})
	if err != nil {
		return err
	}
	return nil
}
