use wasmer::{Instance, Module, Store, BaseTunables, Target, UniversalEngine, Value, imports};

fn main() -> anyhow::Result<()> {
    let module_wat = r#"
    (module
    (type $t0 (func (param i32) (result i32)))
    (func $add_one (export "add_one") (type $t0) (param $p0 i32) (result i32)
        get_local $p0
        i32.const 1
        i32.add))
    "#;

    let engine = UniversalEngine::headless();
    let tunables = BaseTunables::for_target(&Target::default());
    let store = Store::new_with_tunables(&engine, tunables);
    let module = Module::new(&store, &module_wat)?;
    // The module doesn't import anything, so we create an empty import object.
    let import_object = imports! {};
    let instance = Instance::new(&module, &import_object)?;

    let add_one = instance.exports.get_function("add_one")?;
    let result = add_one.call(&[Value::I32(42)]).unwrap();

    print!("Result: {:?}", result);

    Ok(())
}
