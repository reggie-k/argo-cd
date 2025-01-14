package commands

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBearerTokenForHTTPSRepoOnly(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		isHTTPS     bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid HTTPS with Bearer Token",
			bearerToken: "some-token",
			isHTTPS:     true,
			expectError: false,
		},
		{
			name:        "Invalid non-HTTPS with Bearer Token",
			bearerToken: "some-token",
			isHTTPS:     false,
			expectError: true,
			errorMsg:    "--bearer-token is only supported for HTTPS repositories",
		},
		{
			name:        "Valid non-HTTPS without Bearer Token",
			bearerToken: "",
			isHTTPS:     false,
			expectError: false,
		},
		{
			name:        "Valid HTTPS without Bearer Token",
			bearerToken: "",
			isHTTPS:     true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectError {
				// The command for "Invalid non-HTTPS with Bearer Token" case does not return an error.
				// This is because the function validateBearerTokenForHTTPSRepoOnly() does not return an error.
				// Instead, the errors.CheckError(err) performs non-zero code system exit.
				// So in order to test this case, we need to run the command in a separate process.
				// https://stackoverflow.com/a/33404435
				// TODO: consider whether to change all the cmd commands to return an error instead of performing a non-zero code system exit.
				// TODO: generalize this pattern to be used in other tests for failing cmd commands.
				if os.Getenv("BE_CRASHER") == "1" {
					validateBearerTokenForHTTPSRepoOnly(tt.bearerToken, tt.isHTTPS)
					return
				}
				cmd := exec.Command(os.Args[0], "-test.run=TestValidateBearerTokenForHTTPSRepoOnly")
				cmd.Env = append(os.Environ(), "BE_CRASHER=1")
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				err := cmd.Run()
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) && !exitErr.Success() {
					assert.Contains(t, stderr.String(), tt.errorMsg)
					return
				}
				t.Fatalf("process ran with err %v, want exit status 1", err)
			} else {
				// This will never actually panic because the function validateBearerTokenForHTTPSRepoOnly() does not return
				// an error.
				assert.NotPanics(t, func() {
					validateBearerTokenForHTTPSRepoOnly(tt.bearerToken, tt.isHTTPS)
				})
			}
		})
	}
}
