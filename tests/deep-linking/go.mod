module valiant/tests/deep-linking

go 1.25.6

require (
	github.com/stretchr/testify v1.11.1
	valiant v0.0.0
	valiant/tests/common v0.0.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/lib/pq v1.11.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace valiant => ../../backend

replace valiant/tests/common => ../common
