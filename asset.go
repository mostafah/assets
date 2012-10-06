// Package assets prepares CSS and JS files for development and production. It reads,
// processes, and joins asset sources and emits final .css and .js files. "Process"
// means converting LESS and CoffeeScript files into CSS and JS, and compressing
// final files.
//
// API is very simple:
//
//         jsFname, err := assets.New("assets/libraries/*.js", "assets/scripts/app.coffee").Put("static", "name")
//         if err != nil {
//                 log.Fatalln("can't prepare scripts: ", err)
//         }
//
//         cssFname, err := assets.New("assets/style/*.css", "assets/style/*.less").Put("static", "name")
//         if err != nil {
//                 log.Fatalln("can't prepare style files: ", err)
//         }
//
// After the above code, all these asset files are compiled, joined, and compressed
// into single files, put into direcotry "static". You can now pass names of these
// files to your HTML templates.
//
// It also creates two info file in the "static" direcotry to keep track of the
// generated files.
// 
// Compilation and compression of assets are performed by external tools "coffee",
// "lessc", and "yuicompressor", so you should have these tools installed and in your
// PATH if you want to use these features.
package assets

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrNoInput = errors.New("assets: no input file given")
	ErrMix     = errors.New("assets: can't mix CSS and JS in one asset")
)

// type input holds content of each asset source.
type input struct {
	bytes []byte
	// extension of the source file is all the information we need, besides the
	// content of the file
	ext string
}

// type Asset holds a group of assets. You add your asset sources to an Asset, and it
// can process and join them all into a single file which would put where you ask it
// to.
//
// Order of input files are preserved.
//
// Each Asset emits a single .css or .js file. Mixing CSS and JS in one Asset gives an
// error.
type Asset struct {
	filenames       []string // names of the input files
	inputs          []input  // contents of the input files
	hashes          []string // MD5 hash of each input file
	bytes           []byte   // content of output file
	dir, name       string   // dir and name of the asset, passed arguments of Put
	ext             string   // extension, either ".css" or ".js"
	fname, oldfname string   // name of final file
	compress        bool     // does it need compression?
	join            bool     // should join LESS and CoffeeScript before compiling?
}

// New makes an Asset and adds given filenames to it. You can tweak the returned
// asset by adding more files, or just ask it to emit final file by calling Put.
func New(filenames ...string) *Asset {
	a := &Asset{compress: true, join: true}
	a.Add(filenames...)
	return a
}

// Add appends filenames to the Asset a.
func (a *Asset) Add(filenames ...string) {
	a.filenames = append(a.filenames, filenames...)
}

// Put produces final asset file, puts it in dir, and returns its name. Name of the
// file includes the name that's passed as second argument, MD5 hash of the content of
// of the file, and its extention, which is either ".css" or ".js". You can omit the
// name by passing an empty string for it.
func (a *Asset) Put(dir, name string) (fname string, err error) {
	a.dir = dir
	a.name = name
	// expand globs
	if err = a.expandGlobs(); err != nil {
		return
	}
	// check for zero input files
	if len(a.filenames) == 0 {
		return "", ErrNoInput
	}
	// read files into inputs
	if err = a.readInputs(); err != nil {
		return
	}
	// now we know if asset is either ".css" or ".js"
	a.ext = a.inputs[0].ext
	switch a.ext {
	case ".coffee":
		a.ext = ".js"
	case ".less":
		a.ext = ".css"
	}
	if a.ext != ".css" && a.ext != ".js" {
		errMsg := "assets: unsupported extension \"" + a.ext + "\""
		return "", errors.New(errMsg)
	}
	// join LESS and CoffeeScript files before making any progress
	if a.join {
		a.joinFiles()
	}
	// read hashes of inputs
	if err = a.makeHashes(); err != nil {
		return
	}
	// read old info and check if anything has changed
	if changed, err := a.checkSavedInfo(); err != nil || !changed {
		return a.oldfname, err
	}
	// things have changed. delete old files before starting to work
	if err = a.deleteOld(); err != nil {
		return
	}
	// compile LESS and CoffeeSCript
	if err = a.compile(); err != nil {
		return
	}
	// check extensions of all the inputs
	for _, input := range a.inputs {
		if input.ext != a.ext {
			return "", ErrMix
		}
	}
	// join inputs
	for _, input := range a.inputs {
		a.bytes = append(a.bytes, input.bytes...)
	}
	// compress
	if a.compress {
		switch a.ext {
		case ".css":
			a.bytes, err = runCSSCompress(a.bytes)
			if err != nil {
				return
			}
		case ".js":
			a.bytes, err = runJSCompress(a.bytes)
			if err != nil {
				return
			}
		}
	}
	// make filename
	sum, err := hash(a.bytes)
	if err != nil {
		return
	}
	if len(a.name) > 0 {
		a.fname = name + "-"
	}
	a.fname += sum + a.ext
	// create output directory if it does not exists
	if err = os.MkdirAll(dir, 0755); err != nil {
		return
	}
	// save to output file
	err = ioutil.WriteFile(path.Join(dir, a.fname), a.bytes, 0666)
	if err != nil {
		return
	}
	// save asset info files
	if err = a.saveInfo(); err != nil {
		return
	}

	return a.fname, nil
}

// SetCompress enables or disables output compression by yuicompressor. It is enable
// by default. Call SetCompress(false) to disable.
func (a *Asset) SetCompress(compress bool) {
	a.compress = compress
}

// SetJoin can change behaviour of Asset in handling multiple LESS and CoffeeScript
// files. By default, if multiple .less or mulitple .coffee files are provided, Asset
// joins them into a single one before compiling them into CSS and JavaScript. This is
// useful for separating LESS and CoffeeScript code into multiple files. You can
// disable this behavior by setting Join to false.
//
// Please note that Asset should preserve order of input files, so if you provide it
// with
//
//         a.Add("a.coffee", "b.js", "c.coffee", "d.coffee")
//
// only third and fourth files are joined before compilation.
func (a *Asset) SetJoin(join bool) {
	a.join = join
}

// expandGlobs replaces globs in filenames with real file names
func (a *Asset) expandGlobs() error {
	var l []string
	for _, filename := range a.filenames {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return err
		}
		l = append(l, matches...)
	}
	a.filenames = l
	return nil
}

// readInputs loads input files into inputs variable of a.
func (a *Asset) readInputs() error {
	for _, filename := range a.filenames {
		ext := path.Ext(filename)
		bytes, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		a.inputs = append(a.inputs, input{ext: ext, bytes: bytes})
	}
	return nil
}

// joinFiles joins subsequent LESS or CoffeeScript inputs into single ones.
//
// To preserve of the input files, only sequential LESS or CoffeScript files are
// joined as a group. That means that if we have, for example, files "a.coffee",
// "b.js", "c.coffee", and "d.coffee", only third and fourth files are joined.
func (a *Asset) joinFiles() {
	if len(a.inputs) == 0 {
		return
	}
	// LESS or CoffeeScript?
	ext := ""
	switch a.inputs[0].ext {
	case ".js", ".coffee":
		ext = ".coffee"
	case ".css", ".less":
		ext = ".less"
	}
	// can't use range because the list will be changed during the loop
	for i := 0; i < len(a.inputs); i++ {
		// a keeps content of current group of joinable files, starting
		// from file at a.inputs[i]
		bytes := make([]byte, 0)
		n := 0
		for j := i; j < len(a.inputs); j++ {
			if a.inputs[j].ext == ext {
				bytes = append(bytes, a.inputs[j].bytes...)
				n++
			} else {
				// first non-joinable file ends the sequence
				break
			}
		}
		// n == 0 means the current file is not joinable
		// n == 1 means current file is joinable, but its alone
		// both of these situations don't need anything to be done
		if n < 2 {
			continue
		}

		// join all the files
		a.inputs[i].bytes = bytes
		// delete subsequent joined files
		a.inputs = append(a.inputs[:i+1], a.inputs[i+n:]...)
	}
}

// makeHashes generates MD5 hashes of inputs.
func (a *Asset) makeHashes() error {
	for _, inp := range a.inputs {
		sum, err := hash(inp.bytes)
		if err != nil {
			return err
		}
		a.hashes = append(a.hashes, sum)
	}
	return nil
}

// checkSavedInfo loads asset-info file and see if anything has changed or not
func (a *Asset) checkSavedInfo() (chnaged bool, err error) {
	buf, err := ioutil.ReadFile(path.Join(a.dir, a.infoFname()))
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		} else {
			return
		}
	}
	lines := strings.Split(string(buf), "\n")
	if len(lines) < 2 {
		return true, nil
	}
	a.oldfname = lines[0]
	lines = lines[1:]
	if len(lines) != len(a.hashes) {
		return true, nil
	}
	for i, line := range lines {
		if a.hashes[i] != line {
			return true, nil
		}
	}
	return false, nil
}

// deleteOld deletes old asset file and asset info file. This is called before
// generating new file, to keep output directory clean.
func (a *Asset) deleteOld() error {
	if len(a.oldfname) > 0 {
		err := os.Remove(path.Join(a.dir, a.oldfname))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	err := os.Remove(path.Join(a.dir, a.infoFname()))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// compile converts LESS and CoffeeScript inputs to CSS and JS, respectively.
func (a *Asset) compile() error {
	for i := 0; i < len(a.inputs); i++ {
		switch a.inputs[i].ext {
		case ".less":
			b, err := runLess(a.inputs[i].bytes)
			if err != nil {
				return err
			}
			a.inputs[i].bytes = b
			a.inputs[i].ext = ".css"
		case ".coffee":
			b, err := runCoffee(a.inputs[i].bytes)
			if err != nil {
				return err
			}
			a.inputs[i].bytes = b
			a.inputs[i].ext = ".js"
		}
	}
	return nil
}

// saveInfo stores output file name and hashes in info file.
func (a *Asset) saveInfo() error {
	output := a.fname + "\n" + strings.Join(a.hashes, "\n")
	err := ioutil.WriteFile(path.Join(a.dir, a.infoFname()), []byte(output), 0666)
	if err != nil {
		return err
	}
	return nil
}

// infoFname returns name of info file for asset.
func (a *Asset) infoFname() string {
	if len(a.name) > 0 {
		return "asset-info-" + a.name + "-" + a.ext[1:]
	}
	return "asset-info-" + a.ext[1:]
}

// hash returns MD5 hash of r.
func hash(b []byte) (sum string, err error) {
	h := md5.New()
	if _, err = h.Write(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
