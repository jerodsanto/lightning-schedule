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

## Archiving a Season

`go run ./cmd/archive dist/lightning 2025` snapshots the generated site into
`dist/lightning/2025/` (links rewritten, calendar UI removed). Publish by
copying that directory to the server web dir once; the cron never touches it.
Then append the year to `archives` in the program's `config.json` and run
`./deploy.sh` (the config is embedded in the binary) so live pages link to
the new archive in their footer.
