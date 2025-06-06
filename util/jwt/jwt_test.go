package jwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSingleStringScope(t *testing.T) {
	claims := jwt.MapClaims{"groups": "my-org:my-team"}
	groups := GetScopeValues(claims, []string{"groups"})
	assert.Contains(t, groups, "my-org:my-team")
}

func TestGetMultipleListScopes(t *testing.T) {
	claims := jwt.MapClaims{"groups1": []string{"my-org:my-team1"}, "groups2": []string{"my-org:my-team2"}}
	groups := GetScopeValues(claims, []string{"groups1", "groups2"})
	assert.Contains(t, groups, "my-org:my-team1")
	assert.Contains(t, groups, "my-org:my-team2")
}

func TestClaims(t *testing.T) {
	assert.Nil(t, Claims(nil))
	assert.NotNil(t, Claims(jwt.MapClaims{}))
}

func TestIsMember(t *testing.T) {
	assert.False(t, IsMember(jwt.MapClaims{}, nil, []string{"groups"}))
	assert.False(t, IsMember(jwt.MapClaims{"groups": []string{""}}, []string{"my-group"}, []string{"groups"}))
	assert.False(t, IsMember(jwt.MapClaims{"groups": []string{"my-group"}}, []string{""}, []string{"groups"}))
	assert.True(t, IsMember(jwt.MapClaims{"groups": []string{"my-group"}}, []string{"my-group"}, []string{"groups"}))
}

func TestGetGroups(t *testing.T) {
	assert.Empty(t, GetGroups(jwt.MapClaims{}, []string{"groups"}))
	assert.Equal(t, []string{"foo"}, GetGroups(jwt.MapClaims{"groups": []string{"foo"}}, []string{"groups"}))
}

func TestIssuedAtTime_Int64(t *testing.T) {
	// Tuesday, 1 December 2020 14:00:00
	claims := jwt.MapClaims{"iat": int64(1606831200)}
	issuedAt, err := IssuedAtTime(claims)
	require.NoError(t, err)
	str := issuedAt.UTC().Format("Mon Jan _2 15:04:05 2006")
	assert.Equal(t, "Tue Dec  1 14:00:00 2020", str)
}

func TestIssuedAtTime_Error_NoInt(t *testing.T) {
	claims := jwt.MapClaims{"iat": 1606831200}
	_, err := IssuedAtTime(claims)
	assert.Error(t, err)
}

func TestIssuedAtTime_Error_Missing(t *testing.T) {
	claims := jwt.MapClaims{}
	iat, err := IssuedAtTime(claims)
	require.Error(t, err)
	assert.Equal(t, time.Unix(0, 0), iat)
}

func TestIsValid(t *testing.T) {
	assert.True(t, IsValid("foo.bar.foo"))
	assert.True(t, IsValid("foo.bar.foo.bar"))
	assert.False(t, IsValid("foo.bar"))
	assert.False(t, IsValid("foo"))
	assert.False(t, IsValid(""))
}

func TestGetUserIdentifier(t *testing.T) {
	tests := []struct {
		name   string
		claims jwt.MapClaims
		want   string
	}{
		{
			name: "when both dex and sub defined - prefer dex user_id",
			claims: jwt.MapClaims{
				"sub": "ignored:login",
				"federated_claims": map[string]any{
					"user_id": "dex-user",
				},
			},
			want: "dex-user",
		},
		{
			name: "when both dex and sub defined but dex user_id empty - fallback to sub",
			claims: jwt.MapClaims{
				"sub": "test:apiKey",
				"federated_claims": map[string]any{
					"user_id": "",
				},
			},
			want: "test:apiKey",
		},
		{
			name: "when only sub is defined (no dex) - use sub",
			claims: jwt.MapClaims{
				"sub": "admin:login",
			},
			want: "admin:login",
		},
		{
			name:   "when neither dex nor sub defined - return empty",
			claims: jwt.MapClaims{},
			want:   "",
		},
		{
			name:   "nil claims",
			claims: nil,
			want:   "",
		},
		{
			name: "invalid subject",
			claims: jwt.MapClaims{
				"sub": nil,
			},
			want: "",
		},
		{
			name: "invalid federated_claims",
			claims: jwt.MapClaims{
				"sub":              "test:apiKey",
				"federated_claims": "invalid",
			},
			want: "test:apiKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetUserIdentifier(tt.claims)
			assert.Equal(t, tt.want, got)
		})
	}
}
