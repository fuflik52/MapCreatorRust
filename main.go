package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) == 1 {
		runGUI()
		return
	}
	os.Exit(runCLI(os.Args[1:]))
}

func runCLI(args []string) int {
	fs := flag.NewFlagSet("RustMapGen", flag.ContinueOnError)
	size := fs.Int("size", 1500, "world size in Rust map units")
	seed := fs.Int64("seed", 1500111, "procedural seed")
	out := fs.String("out", "", "output .map path")
	fs.Bool("offline", true, "kept for compatibility; offline generation is the default")
	heightRes := fs.Int("height-res", 0, "height/water resolution; default is nextPow2(size/2)+1")
	texRes := fs.Int("tex-res", 0, "splat/biome/alpha/topology resolution; default is nextPow2(size/2)")
	inspect := fs.String("inspect", "", "inspect an existing .map instead of generating")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *inspect != "" {
		summary, err := InspectMap(*inspect)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Print(summary)
		return 0
	}

	output := *out
	if output == "" {
		output = defaultOutputPath(*size, *seed)
	}
	output = filepath.Clean(output)

	result, err := GenerateOfflineMap(OfflineGenConfig{
		Size:      *size,
		Seed:      *seed,
		HeightRes: *heightRes,
		TexRes:    *texRes,
		Out:       output,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Printf("generated %s\n", result.OutputPath)
	if result.Summary != "" {
		fmt.Println(result.Summary)
	}
	return 0
}

func defaultOutputPath(size int, seed int64) string {
	return filepath.Join("out", fmt.Sprintf("proceduralmap.%d.%d.284.map", size, seed))
}
