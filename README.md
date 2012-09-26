## What does it do?

Package `assets` helps you prepare CSS and JS assets for your Go web app. You
give it all your asset source files and it gives you a single, compressed asset
file ready for your website. It can also compile CoffeeScript and LESS files
on-the-fly.

## How to use it

You put your asset files wherever you like and give their address to `assets`:

```
css, err := assets.New("assets/styles/*.less").Put("static", "")
js, err := assets.New("assets/scripts/*.coffee").Put("static", "")
// now you can serve your static folder and pass css and js file names to your
// templates and to call them in your HTML
```

You can disable compression when you are in development mode. Read the
documentation on [GoPkgDoc](http://go.pkgdoc.org/github.com/mostafah/assets).

## Wish list

* It depends on external `coffee`, `lessc`, and `yuicompressor` for compilation
  and compression. Wish there was a better way.
* A way to add more compression and compilation processors: e.g., to add supprot
  for SaSS, or use another JS compressor.
* Better test code.