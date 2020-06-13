package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vektah/dataloaden/pkg/generator"
)

var (
	cache = flag.String("cache", "self", "cache engine.(self, custom)")
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 3 {
		fmt.Println("usage: name keyType valueType")
		fmt.Println(" example:")
		fmt.Println(" dataloaden 'UserLoader int []*github.com/my/package.User'")
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	if err := generator.Generate(args[0], args[1], args[2], wd, *cache); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
}
