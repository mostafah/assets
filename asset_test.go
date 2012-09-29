package assets

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"
)

const (
	outDir = "static"
)

var (
	files = map[string]string{
		"a.css":    "body {\n\tcolor: red;\n}",
		"b.less":   "@c: #444;\n\nb {\ncolor: @c;\n}",
		"a.coffee": "window.a = () -> console.log \"a\"\n",
		"b.coffee": "window.b = () -> console.log \"b\"\n",
		"c.js":     "window.c = function(){console.log(\"c\");};\n",
	}
)

type assetTest struct {
	name   string
	files  []string
	output string
}

func TestAssets(t *testing.T) {
	cssTest := assetTest{
		"test",
		[]string{"a.css", "b.less"},
		"body{color:red}b{color:#444}",
	}
	jsTest := assetTest{
		"test",
		[]string{"a.coffee", "b.coffee"},
		"(function(){window.a=function(){return console.log(\"a\")};" +
			"window.b=function(){return console.log(\"b\")}}).call(this);",
	}
	jsTest2 := assetTest{
		"test",
		[]string{"c.js"},
		"window.c=function(){console.log(\"c\")};",
	}
	globTest := assetTest{
		"",
		[]string{"*.coffee"},
		"(function(){window.a=function(){return console.log(\"a\")};" +
			"window.b=function(){return console.log(\"b\")}}).call(this);",
	}

	// create a template directory and change to that
	dir, err := ioutil.TempDir(os.TempDir(), "asset_test")
	if err != nil {
		log.Fatalf("can't create temp directory: %v\n", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		log.Fatalf("can't cd to directory \"%s\": %v\n", dir, err)
	}

	// create test files
	for name, content := range files {
		err = ioutil.WriteFile(name, []byte(content), 0644)
		if err != nil {
			log.Fatalf("can't create test file \"%s\": %v\n", name, err)
		}
	}

	doTest(cssTest, t)
	fname := doTest(jsTest, t)
	doTest(jsTest2, t)
	// now first output should be removed
	if exists(path.Join("static", fname)) {
		t.Fatalf("Put failed to remove old file \"%s\".", fname)
	}
	doTest(globTest, t)
}

func doTest(test assetTest, t *testing.T) (fname string) {
	a := New(test.files...)
	fname, err := a.Put(outDir, test.name)
	if err != nil {
		t.Fatalf("Put returned error: %v\n", err)
		return fname
	}

	buf, err := ioutil.ReadFile(path.Join("static", fname))
	if string(buf) != test.output {
		t.Fatalf("expected: %s\ngot: %s\n", test.output, string(buf))
	}

	return fname
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	log.Fatalf("can't check for existence of file \"%s\": %v\n", path, err)
	return false
}
