package main

import (
	k6conv "github.com/deicon/httprunner/src/converter/k6"
	chttp "github.com/deicon/httprunner/src/http"
)

// requireK6Generate is isolated to avoid import when not used
func requireK6Generate(requests []chttp.Request, opts struct {
	Iterations, DelayMS int
	EnvFile             string
}) (string, error) {
	return k6conv.Generate(requests, k6conv.Options{
		Iterations: opts.Iterations,
		DelayMS:    opts.DelayMS,
		EnvFile:    opts.EnvFile,
	})
}
