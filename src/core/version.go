package core

// Version is the application version shown in the GUI and used by build
// scripts in artifact names. It is a var rather than a const so release builds
// can override it with Go ldflags when CI tags a build.
var Version = "0.2.4"
