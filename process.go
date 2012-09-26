package assets

import (
	"errors"
	"github.com/mostafah/run"
)

func runLess(in []byte) (out []byte, err error) {
	return runCmd(in, "lessc", "-")
}

func runCoffee(in []byte) (out []byte, err error) {
	return runCmd(in, "coffee", "-sc")
}

func runCSSCompress(in []byte) (out []byte, err error) {
	return runCmd(in, "yuicompressor", "--type", "css")
}

func runJSCompress(in []byte) (out []byte, err error) {
	return runCmd(in, "yuicompressor", "--type", "js")
}

func runCmd(in []byte, cmd string, args ...string) (out []byte, err error) {
	stdout, stderr, err := run.Run(in, cmd, args...)
	if len(stderr) != 0 {
		return nil, errors.New("stderr:" + string(stderr))
	} else if err != nil {
		return nil, err
	}
	return stdout, nil
}
