//go:build ignore

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/shurcooL/vfsgen"
)

func packageAssets(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &os.PathError{
			Op:   "stat",
			Path: dir,
			Err:  os.ErrInvalid,
		}
	}

	assets := http.Dir(dir)
	return vfsgen.Generate(assets, vfsgen.Options{
		Filename:     "cmd/assets/generated_assets.go",
		PackageName:  "assets",
		VariableName: "VirtualFS",
		BuildTags:    "withAssets",
	})
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run generate.go <dir>")
	}

	err := packageAssets(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to package assets: %v", err)
	}
}
