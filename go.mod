module github.com/example/enterprise-scheduler

go 1.24.0

toolchain go1.24.3

require modernc.org/sqlite v0.0.0

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
)

replace modernc.org/sqlite => ./modernc.org/sqlite
