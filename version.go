package ssw

import (
	"fmt"
	"runtime"
)

// Version contains all the necessary information about a build
type Version struct {
	// Name of the application
	Name string

	// Description is the description of the application
	Description string

	// Version is the version of the build
	Version string

	// Hash is the hash of the version
	Hash string

	// Date is the time of when the application was built
	Date string

	// Clean is the binary built from a clean source tree
	Clean string
}

// NewVersion returns a new Version
func NewVersion(name string, desc string, ver string, hash string, date string, clean string) *Version {
	return &Version{
		Name:        name,
		Description: desc,
		Version:     ver,
		Hash:        hash,
		Date:        date,
		Clean:       clean,
	}
}

// GetVersion gets the version string formatted
func (bd *Version) GetVersion() string {

	return fmt.Sprintf(
		"%s version %s build %s (%s), built on %s, with %s",
		bd.Name,
		bd.Version,
		bd.Hash,
		runtime.GOARCH,
		bd.Date,
		runtime.Version(),
	)

}

// GetDetails gets the formatted details
func (bd *Version) GetDetails() string {

	var detailString = `Name: %s
Description: %s
Version: %s
Git Commit Hash: %s
Build date: %s
Built from clean source tree: %s
OS: %s
Architecture: %s
Go version: %s`

	return fmt.Sprintf(detailString,
		bd.Name,
		bd.Description,
		bd.Version,
		bd.Hash,
		bd.Date,
		bd.Clean,
		runtime.GOOS,
		runtime.GOARCH,
		runtime.Version(),
	)

}
