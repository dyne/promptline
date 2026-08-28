module promptline

go 1.24.0

// Keep source language semantics at Go 1.24 while building with a supported,
// patched compiler. Update this only through the documented toolchain policy.
toolchain go1.25.12

require (
	github.com/go-playground/validator/v10 v10.28.0
	github.com/invopop/jsonschema v0.13.0
	golang.org/x/sys v0.36.0
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/u-root/u-root v0.15.0

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.10 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)
