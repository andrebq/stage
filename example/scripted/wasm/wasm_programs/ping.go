//go:generate
package main

func stage_exports_print_number(value int)

func main() {}

//export actor_main
func actor_main() int {
	stage_exports_print_number(10)
	return 0
}
