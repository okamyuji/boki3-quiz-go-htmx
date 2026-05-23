// Package version exposes the application build version.
// The value is overridden at link time with -ldflags "-X .../version.value=v1.2.3".
package version

var value = "dev"

// String returns the build version.
func String() string {
	return value
}
