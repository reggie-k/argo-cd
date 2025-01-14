package commands

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBearerTokenAndPasswordCombo(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		password    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Both token and password set",
			bearerToken: "some-token",
			password:    "some-password",
			expectError: true,
			errorMsg:    "only --bearer-token or --password is allowed, not both",
		},
		{
			name:        "Only token set",
			bearerToken: "some-token",
			password:    "",
			expectError: false,
		},
		{
			name:        "Only password set",
			bearerToken: "",
			password:    "some-password",
			expectError: false,
		},
		{
			name:        "Neither token nor password set",
			bearerToken: "",
			password:    "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectError {
				// The command for "Invalid non-HTTPS with Bearer Token" case does not return an error.
				// This is because the function validateBearerTokenAndPasswordCombo() does not return an error.
				// Instead, the errors.CheckError(err) performs non-zero code system exit.
				// So in order to test this case, we need to run the command in a separate process.
				// https://stackoverflow.com/a/33404435
				// TODO: consider whether to change all the cmd commands to return an error instead of performing a non-zero code system exit.
				// TODO: generalize this pattern to be used in other tests for failing cmd commands.
				if os.Getenv("BE_CRASHER") == "1" {
					validateBearerTokenAndPasswordCombo(tt.bearerToken, tt.password)
					return
				}
				cmd := exec.Command(os.Args[0], "-test.run=TestValidateBearerTokenAndPasswordCombo")
				cmd.Env = append(os.Environ(), "BE_CRASHER=1")
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				err := cmd.Run()
				if e, ok := err.(*exec.ExitError); ok && !e.Success() {
					assert.Contains(t, stderr.String(), tt.errorMsg)
					return
				}
				t.Fatalf("process ran with err %v, want exit status 1", err)
			} else {
				// This will never actually panic because the function validateBearerTokenAndPasswordCombo() does not return
				// an error.
				assert.NotPanics(t, func() {
					validateBearerTokenAndPasswordCombo(tt.bearerToken, tt.password)
				})
			}
		})
	}
}
