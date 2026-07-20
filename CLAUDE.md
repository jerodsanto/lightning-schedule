## Testing Output

Whenever you want to test results, run:

`go run generate.go`

and test the output in the `dist/lightning` folder (the `-program` flag defaults
to `lightning`; use `-program warriors` → `dist/warriors` for the Warriors
program). Do not generate additional binaries or output directories.

## Adding a Program

Create `programs/<name>/` with `config.json`, `theme.css`, `icon.png`, and
`favicon.png` (copy `programs/lightning/` as a template), run `./build.sh`,
then add the program and its web dir to `deploy.sh`.
