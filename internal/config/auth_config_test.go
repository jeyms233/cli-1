package config

import (
	"errors"
	"testing"

	ghConfig "github.com/cli/go-gh/v2/pkg/config"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func newTestAuthConfig() *AuthConfig {
	return &AuthConfig{
		cfg: ghConfig.ReadFromString(""),
	}
}

func TestTokenFromKeyring(t *testing.T) {
	// Given a keyring that contains a token for a host
	keyring.MockInit()
	require.NoError(t, keyring.Set(keyringServiceName("github.com"), "", "test-token"))

	// When we get the token from the auth config
	authCfg := newTestAuthConfig()
	token, err := authCfg.TokenFromKeyring("github.com")

	// Then it returns successfully with the correct token
	require.NoError(t, err)
	require.Equal(t, "test-token", token)
}

func TestTokenStoredInConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// When the user has logged in insecurely
	authCfg := newTestAuthConfig()
	ghConfig.Read = func() (*ghConfig.Config, error) {
		return authCfg.cfg, nil
	}
	_, err := authCfg.Login("github.com", "test-user", "test-token", "", false)
	require.NoError(t, err)

	// When we get the token
	token, source := authCfg.Token("github.com")

	// Then the token is successfully fetched
	// and the source is set to oauth_token but this isn't great
	// but I can't find the issue # that references this.
	require.Equal(t, "test-token", token)
	require.Equal(t, "oauth_token", source)
}

func TestTokenStoredInEnv(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// When the user is authenticated via env var
	authCfg := newTestAuthConfig()
	ghConfig.Read = func() (*ghConfig.Config, error) {
		return authCfg.cfg, nil
	}
	t.Setenv("GH_TOKEN", "test-token")

	// When we get the token
	token, source := authCfg.Token("github.com")

	// Then the token is successfully fetched
	// and the source is set to the name of the env var
	require.Equal(t, "test-token", token)
	require.Equal(t, "GH_TOKEN", source)
}

func TestTokenStoredInKeyring(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// When the user has logged in securely
	keyring.MockInit()
	authCfg := newTestAuthConfig()
	ghConfig.Read = func() (*ghConfig.Config, error) {
		return authCfg.cfg, nil
	}
	_, err := authCfg.Login("github.com", "test-user", "test-token", "", true)
	require.NoError(t, err)

	// When we get the token
	token, source := authCfg.Token("github.com")

	// Then the token is successfully fetched
	// and the source is set to keyring
	require.Equal(t, "test-token", token)
	require.Equal(t, "keyring", source)
}

func TestTokenFromKeyringNonExistent(t *testing.T) {
	// Given a keyring that doesn't contain any tokens
	keyring.MockInit()

	// When we try to get a token from the auth config
	authCfg := newTestAuthConfig()
	_, err := authCfg.TokenFromKeyring("github.com")

	// Then it returns failure bubbling the ErrNotFound
	require.ErrorIs(t, err, keyring.ErrNotFound)
}

func TestHasEnvTokenWithoutAnyEnvToken(t *testing.T) {
	// Given an empty hosts configuration
	authCfg := newTestAuthConfig()
	ghConfig.Read = func() (*ghConfig.Config, error) {
		return authCfg.cfg, nil
	}

	// When we check if it has an env token
	hasEnvToken := authCfg.HasEnvToken()

	// Then it returns false
	require.False(t, hasEnvToken, "expected not to have env token")
}

func TestHasEnvTokenWithEnvToken(t *testing.T) {
	// Given an empty hosts configuration but a token set in the env var
	authCfg := newTestAuthConfig()
	ghConfig.Read = func() (*ghConfig.Config, error) {
		return authCfg.cfg, nil
	}
	t.Setenv("GH_ENTERPRISE_TOKEN", "test-token")

	// When we check if it has an env token
	hasEnvToken := authCfg.HasEnvToken()

	// Then it returns true
	require.True(t, hasEnvToken, "expected to have env token")
}

func TestHasEnvTokenWithNoEnvTokenButAConfigVar(t *testing.T) {
	t.Skip("this test is explicitly breaking some implementation assumptions")

	// Given a token in the config
	authCfg := newTestAuthConfig()
	ghConfig.Read = func() (*ghConfig.Config, error) {
		return authCfg.cfg, nil
	}
	// Using example.com here will cause the token to be returned from the config
	_, err := authCfg.Login("example.com", "test-user", "test-token", "", false)
	require.NoError(t, err)

	// When we check if it has an env token
	hasEnvToken := authCfg.HasEnvToken()

	// Then it SHOULD return false
	require.False(t, hasEnvToken, "expected not to have env token")
}

func TestUserNotLoggedIn(t *testing.T) {
	// Given we have not logged in
	authCfg := newTestAuthConfig()

	// When we get the user
	_, err := authCfg.User("github.com")

	// Then it returns failure, bubbling the KeyNotFoundError
	var keyNotFoundError *ghConfig.KeyNotFoundError
	require.ErrorAs(t, err, &keyNotFoundError)
}

func TestNoGitProtocolInAuthConfig(t *testing.T) {
	// Given a host configuration without a git protocol
	authCfg := newTestAuthConfig()

	// When we get the git protocol
	gitProtocol, err := authCfg.GitProtocol("github.com")

	// Then it returns success, using the default
	require.NoError(t, err)
	require.Equal(t, "https", gitProtocol)
}

func TestGitProtocolInAuthConfig(t *testing.T) {
	// Given an a host configuration with a git protocol
	authCfg := newTestAuthConfig()
	authCfg.cfg.Set([]string{hosts, "github.com", "git_protocol"}, "ssh")

	// When we get the git protocol
	gitProtocol, err := authCfg.GitProtocol("github.com")

	// Then it returns success with the correct git protocol
	require.NoError(t, err)
	require.Equal(t, "ssh", gitProtocol)
}

func TestLoginSecureStorageUsesKeyring(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given a usable keyring
	keyring.MockInit()
	authCfg := newTestAuthConfig()

	// When we login with secure storage
	insecureStorageUsed, err := authCfg.Login("github.com", "test-user", "test-token", "", true)

	// Then it returns success, notes that insecure storage was not used, and stores the token in the keyring
	require.NoError(t, err)
	require.False(t, insecureStorageUsed, "expected to use secure storage")

	token, err := keyring.Get(keyringServiceName("github.com"), "")
	require.NoError(t, err)
	require.Equal(t, "test-token", token)
}

func TestLoginSecureStorageRemovesOldInsecureConfigToken(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given a usable keyring and an oauth token in the config
	keyring.MockInit()
	authCfg := newTestAuthConfig()
	authCfg.cfg.Set([]string{hosts, "github.com", oauthToken}, "old-token")

	// When we login with secure storage
	_, err := authCfg.Login("github.com", "test-user", "test-token", "", true)

	// Then it returns success, having also removed the old token from the config
	require.NoError(t, err)
	requireNoKey(t, authCfg.cfg, []string{hosts, "github.com", oauthToken})
}

func TestLoginSecureStorageWithErrorFallsbackAndReports(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given a keyring that errors
	keyring.MockInitWithError(errors.New("test-explosion"))
	authCfg := newTestAuthConfig()

	// When we login with secure storage
	insecureStorageUsed, err := authCfg.Login("github.com", "test-user", "test-token", "", true)

	// Then it returns success, reports that insecure storage was used, and stores the token in the config
	require.NoError(t, err)

	require.True(t, insecureStorageUsed, "expected to use insecure storage")
	requireKeyWithValue(t, authCfg.cfg, []string{hosts, "github.com", oauthToken}, "test-token")
}

func TestLoginInsecureStorage(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	authCfg := newTestAuthConfig()

	// When we login with insecure storage
	insecureStorageUsed, err := authCfg.Login("github.com", "test-user", "test-token", "", false)

	// Then it returns success, notes that insecure storage was used, and stores the token in the config
	require.NoError(t, err)

	require.True(t, insecureStorageUsed, "expected to use insecure storage")
	requireKeyWithValue(t, authCfg.cfg, []string{hosts, "github.com", oauthToken}, "test-token")
}

func TestLoginSetsUserForProvidedHost(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given a usable keyring and an empty config
	keyring.MockInit()
	authCfg := newTestAuthConfig()

	// When we login
	_, err := authCfg.Login("github.com", "test-user", "test-token", "ssh", true)

	// Then it returns success and the user is set
	require.NoError(t, err)

	user, err := authCfg.User("github.com")
	require.NoError(t, err)
	require.Equal(t, "test-user", user)
}

func TestLoginSetsGitProtocolForProdivdedHost(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given a usable keyring and an empty config
	keyring.MockInit()
	authCfg := newTestAuthConfig()

	// When we login
	_, err := authCfg.Login("github.com", "test-user", "test-token", "ssh", true)

	// Then it returns success and the git protocol is set
	require.NoError(t, err)

	gitProtocol, err := authCfg.GitProtocol("github.com")
	require.NoError(t, err)
	require.Equal(t, "ssh", gitProtocol)
}

func TestLoginAddsHostIfNotAlreadyAdded(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given a usable keyring and an empty config
	keyring.MockInit()
	authCfg := newTestAuthConfig()
	ghConfig.Read = func() (*ghConfig.Config, error) {
		return authCfg.cfg, nil
	}

	// When we login
	_, err := authCfg.Login("github.com", "test-user", "test-token", "ssh", true)

	// Then it returns success and a host is added
	require.NoError(t, err)

	hosts := authCfg.Hosts()
	require.Contains(t, hosts, "github.com")
}

func TestLogoutRemovesHostAndKeyringToken(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given we are logged into a host
	keyring.MockInit()
	authCfg := newTestAuthConfig()
	_, err := authCfg.Login("github.com", "test-user", "test-token", "ssh", true)
	require.NoError(t, err)

	// When we logout
	err = authCfg.Logout("github.com")

	// Then we return success, and the host and token are removed from the config and keyring
	require.NoError(t, err)

	requireNoKey(t, authCfg.cfg, []string{hosts, "github.com"})
	_, err = keyring.Get(keyringServiceName("github.com"), "")
	require.ErrorIs(t, err, keyring.ErrNotFound)
}

// Note that I'm not sure this test enforces particularly desirable behaviour
// since it leads users to believe a token has been removed when really
// that might have failed for some reason.
//
// The original intention here is that if the logout fails, the user can't
// really do anything to recover. On the other hand, a user might
// want to rectify this manually, for example if there were on a shared machine.
func TestLogoutIgnoresErrorsFromConfigAndKeyring(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)

	// Given we have keyring that errors, and a config that
	// doesn't even have a hosts key (which would cause Remove to fail)
	keyring.MockInitWithError(errors.New("test-explosion"))
	authCfg := newTestAuthConfig()

	// When we logout
	err := authCfg.Logout("github.com")

	// Then it returns success anyway, suppressing the errors
	require.NoError(t, err)
}

func requireKeyWithValue(t *testing.T, cfg *ghConfig.Config, keys []string, value string) {
	t.Helper()

	actual, err := cfg.Get(keys)
	require.NoError(t, err)

	require.Equal(t, value, actual)
}

func requireNoKey(t *testing.T, cfg *ghConfig.Config, keys []string) {
	t.Helper()

	_, err := cfg.Get(keys)
	var keyNotFoundError *ghConfig.KeyNotFoundError
	require.ErrorAs(t, err, &keyNotFoundError)
}
