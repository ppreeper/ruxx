// Package e2e holds end-to-end acceptance tests. They build the real ruxx
// binary and drive it as a user would: through the command line, in a
// temporary directory, asserting stdout/stderr and exit codes.
//
// The unit tests in internal/cli call cli.Run in-process; these tests exist to
// catch what that cannot see -- flag parsing as the process sees it, real file
// writes, environment variables, and the exit-code contract.
package e2e
