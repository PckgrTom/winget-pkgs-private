# winget-pkgs-private

- `winget source add -n pckgr https://winget-pkgs-private.vercel.app/api -t Microsoft.Rest`

Limitations:

- Only returns latest version.
- Only returns default locale.

TODO:

- Add `DATABASE_URL` secret (Github)
- Add `GH_PAT` secret (Github)
- Add `GITHUB_PAT` secret (Vercel)
