package main

import (
	"context"
	"crypto"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc"
	"github.com/dgrijalva/jwt-go"
	"github.com/mbland/hmacauth"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/apis/options"
	sessionsapi "github.com/oauth2-proxy/oauth2-proxy/pkg/apis/sessions"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/encryption"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/logger"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/requests"
	"github.com/oauth2-proxy/oauth2-proxy/pkg/sessions"
	"github.com/oauth2-proxy/oauth2-proxy/providers"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Options holds Configuration Options that can be set by Command Line Flag,
// or Config File
type Options struct {
	ProxyPrefix      string `flag:"proxy-prefix" cfg:"proxy_prefix" env:"OAUTH2_PROXY_PROXY_PREFIX"`
	PingPath         string `flag:"ping-path" cfg:"ping_path" env:"OAUTH2_PROXY_PING_PATH"`
	ProxyWebSockets  bool   `flag:"proxy-websockets" cfg:"proxy_websockets" env:"OAUTH2_PROXY_PROXY_WEBSOCKETS"`
	HTTPAddress      string `flag:"http-address" cfg:"http_address" env:"OAUTH2_PROXY_HTTP_ADDRESS"`
	HTTPSAddress     string `flag:"https-address" cfg:"https_address" env:"OAUTH2_PROXY_HTTPS_ADDRESS"`
	ReverseProxy     bool   `flag:"reverse-proxy" cfg:"reverse_proxy" env:"OAUTH2_PROXY_REVERSE_PROXY"`
	ForceHTTPS       bool   `flag:"force-https" cfg:"force_https" env:"OAUTH2_PROXY_FORCE_HTTPS"`
	RedirectURL      string `flag:"redirect-url" cfg:"redirect_url" env:"OAUTH2_PROXY_REDIRECT_URL"`
	ClientID         string `flag:"client-id" cfg:"client_id" env:"OAUTH2_PROXY_CLIENT_ID"`
	ClientSecret     string `flag:"client-secret" cfg:"client_secret" env:"OAUTH2_PROXY_CLIENT_SECRET"`
	ClientSecretFile string `flag:"client-secret-file" cfg:"client_secret_file" env:"OAUTH2_PROXY_CLIENT_SECRET_FILE"`
	TLSCertFile      string `flag:"tls-cert-file" cfg:"tls_cert_file" env:"OAUTH2_PROXY_TLS_CERT_FILE"`
	TLSKeyFile       string `flag:"tls-key-file" cfg:"tls_key_file" env:"OAUTH2_PROXY_TLS_KEY_FILE"`

	AuthenticatedEmailsFile  string   `flag:"authenticated-emails-file" cfg:"authenticated_emails_file" env:"OAUTH2_PROXY_AUTHENTICATED_EMAILS_FILE"`
	KeycloakGroup            string   `flag:"keycloak-group" cfg:"keycloak_group" env:"OAUTH2_PROXY_KEYCLOAK_GROUP"`
	AzureTenant              string   `flag:"azure-tenant" cfg:"azure_tenant" env:"OAUTH2_PROXY_AZURE_TENANT"`
	BitbucketTeam            string   `flag:"bitbucket-team" cfg:"bitbucket_team" env:"OAUTH2_PROXY_BITBUCKET_TEAM"`
	BitbucketRepository      string   `flag:"bitbucket-repository" cfg:"bitbucket_repository" env:"OAUTH2_PROXY_BITBUCKET_REPOSITORY"`
	EmailDomains             []string `flag:"email-domain" cfg:"email_domains" env:"OAUTH2_PROXY_EMAIL_DOMAINS"`
	WhitelistDomains         []string `flag:"whitelist-domain" cfg:"whitelist_domains" env:"OAUTH2_PROXY_WHITELIST_DOMAINS"`
	GitHubOrg                string   `flag:"github-org" cfg:"github_org" env:"OAUTH2_PROXY_GITHUB_ORG"`
	GitHubTeam               string   `flag:"github-team" cfg:"github_team" env:"OAUTH2_PROXY_GITHUB_TEAM"`
	GitLabGroup              string   `flag:"gitlab-group" cfg:"gitlab_group" env:"OAUTH2_PROXY_GITLAB_GROUP"`
	GoogleGroups             []string `flag:"google-group" cfg:"google_group" env:"OAUTH2_PROXY_GOOGLE_GROUPS"`
	GoogleAdminEmail         string   `flag:"google-admin-email" cfg:"google_admin_email" env:"OAUTH2_PROXY_GOOGLE_ADMIN_EMAIL"`
	GoogleServiceAccountJSON string   `flag:"google-service-account-json" cfg:"google_service_account_json" env:"OAUTH2_PROXY_GOOGLE_SERVICE_ACCOUNT_JSON"`
	HtpasswdFile             string   `flag:"htpasswd-file" cfg:"htpasswd_file" env:"OAUTH2_PROXY_HTPASSWD_FILE"`
	DisplayHtpasswdForm      bool     `flag:"display-htpasswd-form" cfg:"display_htpasswd_form" env:"OAUTH2_PROXY_DISPLAY_HTPASSWD_FORM"`
	CustomTemplatesDir       string   `flag:"custom-templates-dir" cfg:"custom_templates_dir" env:"OAUTH2_PROXY_CUSTOM_TEMPLATES_DIR"`
	Banner                   string   `flag:"banner" cfg:"banner" env:"OAUTH2_PROXY_BANNER"`
	Footer                   string   `flag:"footer" cfg:"footer" env:"OAUTH2_PROXY_FOOTER"`

	Cookie  options.CookieOptions  `cfg:",squash"`
	Session options.SessionOptions `cfg:",squash"`

	Upstreams                     []string      `flag:"upstream" cfg:"upstreams" env:"OAUTH2_PROXY_UPSTREAMS"`
	SkipAuthRegex                 []string      `flag:"skip-auth-regex" cfg:"skip_auth_regex" env:"OAUTH2_PROXY_SKIP_AUTH_REGEX"`
	SkipAuthHeader                []string      `flag:"skip-auth-header" cfg:"skip_auth_header"`
	SkipJwtBearerTokens           bool          `flag:"skip-jwt-bearer-tokens" cfg:"skip_jwt_bearer_tokens" env:"OAUTH2_PROXY_SKIP_JWT_BEARER_TOKENS"`
	ExtraJwtIssuers               []string      `flag:"extra-jwt-issuers" cfg:"extra_jwt_issuers" env:"OAUTH2_PROXY_EXTRA_JWT_ISSUERS"`
	PassBasicAuth                 bool          `flag:"pass-basic-auth" cfg:"pass_basic_auth" env:"OAUTH2_PROXY_PASS_BASIC_AUTH"`
	SetBasicAuth                  bool          `flag:"set-basic-auth" cfg:"set_basic_auth" env:"OAUTH2_PROXY_SET_BASIC_AUTH"`
	PreferEmailToUser             bool          `flag:"prefer-email-to-user" cfg:"prefer_email_to_user" env:"OAUTH2_PROXY_PREFER_EMAIL_TO_USER"`
	BasicAuthPassword             string        `flag:"basic-auth-password" cfg:"basic_auth_password" env:"OAUTH2_PROXY_BASIC_AUTH_PASSWORD"`
	PassAccessToken               bool          `flag:"pass-access-token" cfg:"pass_access_token" env:"OAUTH2_PROXY_PASS_ACCESS_TOKEN"`
	PassHostHeader                bool          `flag:"pass-host-header" cfg:"pass_host_header" env:"OAUTH2_PROXY_PASS_HOST_HEADER"`
	SkipProviderButton            bool          `flag:"skip-provider-button" cfg:"skip_provider_button" env:"OAUTH2_PROXY_SKIP_PROVIDER_BUTTON"`
	PassUserHeaders               bool          `flag:"pass-user-headers" cfg:"pass_user_headers" env:"OAUTH2_PROXY_PASS_USER_HEADERS"`
	SSLInsecureSkipVerify         bool          `flag:"ssl-insecure-skip-verify" cfg:"ssl_insecure_skip_verify" env:"OAUTH2_PROXY_SSL_INSECURE_SKIP_VERIFY"`
	SSLUpstreamInsecureSkipVerify bool          `flag:"ssl-upstream-insecure-skip-verify" cfg:"ssl_upstream_insecure_skip_verify" env:"OAUTH2_PROXY_SSL_UPSTREAM_INSECURE_SKIP_VERIFY"`
	SetXAuthRequest               bool          `flag:"set-xauthrequest" cfg:"set_xauthrequest" env:"OAUTH2_PROXY_SET_XAUTHREQUEST"`
	SetAuthorization              bool          `flag:"set-authorization-header" cfg:"set_authorization_header" env:"OAUTH2_PROXY_SET_AUTHORIZATION_HEADER"`
	PassAuthorization             bool          `flag:"pass-authorization-header" cfg:"pass_authorization_header" env:"OAUTH2_PROXY_PASS_AUTHORIZATION_HEADER"`
	SkipAuthPreflight             bool          `flag:"skip-auth-preflight" cfg:"skip_auth_preflight" env:"OAUTH2_PROXY_SKIP_AUTH_PREFLIGHT"`
	FlushInterval                 time.Duration `flag:"flush-interval" cfg:"flush_interval" env:"OAUTH2_PROXY_FLUSH_INTERVAL"`

	// These options allow for other providers besides Google, with
	// potential overrides.
	Provider                           string `flag:"provider" cfg:"provider" env:"OAUTH2_PROXY_PROVIDER"`
	ProviderName                       string `flag:"provider-display-name" cfg:"provider_display_name" env:"OAUTH2_PROXY_PROVIDER_DISPLAY_NAME"`
	OIDCIssuerURL                      string `flag:"oidc-issuer-url" cfg:"oidc_issuer_url" env:"OAUTH2_PROXY_OIDC_ISSUER_URL"`
	InsecureOIDCAllowUnverifiedEmail   bool   `flag:"insecure-oidc-allow-unverified-email" cfg:"insecure_oidc_allow_unverified_email" env:"OAUTH2_PROXY_INSECURE_OIDC_ALLOW_UNVERIFIED_EMAIL"`
	InsecureOIDCSkipIssuerVerification bool   `flag:"insecure-oidc-skip-issuer-verification" cfg:"insecure_oidc_skip_issuer_verification" env:"OAUTH2_PROXY_INSECURE_OIDC_SKIP_ISSUER_VERIFICATION"`
	SkipOIDCDiscovery                  bool   `flag:"skip-oidc-discovery" cfg:"skip_oidc_discovery" env:"OAUTH2_PROXY_SKIP_OIDC_DISCOVERY"`
	OIDCJwksURL                        string `flag:"oidc-jwks-url" cfg:"oidc_jwks_url" env:"OAUTH2_PROXY_OIDC_JWKS_URL"`
	LoginURL                           string `flag:"login-url" cfg:"login_url" env:"OAUTH2_PROXY_LOGIN_URL"`
	RedeemURL                          string `flag:"redeem-url" cfg:"redeem_url" env:"OAUTH2_PROXY_REDEEM_URL"`
	ProfileURL                         string `flag:"profile-url" cfg:"profile_url" env:"OAUTH2_PROXY_PROFILE_URL"`
	ProtectedResource                  string `flag:"resource" cfg:"resource" env:"OAUTH2_PROXY_RESOURCE"`
	ValidateURL                        string `flag:"validate-url" cfg:"validate_url" env:"OAUTH2_PROXY_VALIDATE_URL"`
	Scope                              string `flag:"scope" cfg:"scope" env:"OAUTH2_PROXY_SCOPE"`
	Prompt                             string `flag:"prompt" cfg:"prompt" env:"OAUTH2_PROXY_PROMPT"`
	ApprovalPrompt                     string `flag:"approval-prompt" cfg:"approval_prompt" env:"OAUTH2_PROXY_APPROVAL_PROMPT"` // Deprecated by OIDC 1.0
	UserIDClaim                        string `flag:"user-id-claim" cfg:"user_id_claim" env:"OAUTH2_PROXY_USER_ID_CLAIM"`

	// Configuration values for logging
	LoggingFilename       string `flag:"logging-filename" cfg:"logging_filename" env:"OAUTH2_PROXY_LOGGING_FILENAME"`
	LoggingMaxSize        int    `flag:"logging-max-size" cfg:"logging_max_size" env:"OAUTH2_PROXY_LOGGING_MAX_SIZE"`
	LoggingMaxAge         int    `flag:"logging-max-age" cfg:"logging_max_age" env:"OAUTH2_PROXY_LOGGING_MAX_AGE"`
	LoggingMaxBackups     int    `flag:"logging-max-backups" cfg:"logging_max_backups" env:"OAUTH2_PROXY_LOGGING_MAX_BACKUPS"`
	LoggingLocalTime      bool   `flag:"logging-local-time" cfg:"logging_local_time" env:"OAUTH2_PROXY_LOGGING_LOCAL_TIME"`
	LoggingCompress       bool   `flag:"logging-compress" cfg:"logging_compress" env:"OAUTH2_PROXY_LOGGING_COMPRESS"`
	StandardLogging       bool   `flag:"standard-logging" cfg:"standard_logging" env:"OAUTH2_PROXY_STANDARD_LOGGING"`
	StandardLoggingFormat string `flag:"standard-logging-format" cfg:"standard_logging_format" env:"OAUTH2_PROXY_STANDARD_LOGGING_FORMAT"`
	RequestLogging        bool   `flag:"request-logging" cfg:"request_logging" env:"OAUTH2_PROXY_REQUEST_LOGGING"`
	RequestLoggingFormat  string `flag:"request-logging-format" cfg:"request_logging_format" env:"OAUTH2_PROXY_REQUEST_LOGGING_FORMAT"`
	ExcludeLoggingPaths   string `flag:"exclude-logging-paths" cfg:"exclude_logging_paths" env:"OAUTH2_PROXY_EXCLUDE_LOGGING_PATHS"`
	SilencePingLogging    bool   `flag:"silence-ping-logging" cfg:"silence_ping_logging" env:"OAUTH2_PROXY_SILENCE_PING_LOGGING"`
	AuthLogging           bool   `flag:"auth-logging" cfg:"auth_logging" env:"OAUTH2_PROXY_LOGGING_AUTH_LOGGING"`
	AuthLoggingFormat     string `flag:"auth-logging-format" cfg:"auth_logging_format" env:"OAUTH2_PROXY_AUTH_LOGGING_FORMAT"`
	SignatureKey          string `flag:"signature-key" cfg:"signature_key" env:"OAUTH2_PROXY_SIGNATURE_KEY"`
	AcrValues             string `flag:"acr-values" cfg:"acr_values" env:"OAUTH2_PROXY_ACR_VALUES"`
	JWTKey                string `flag:"jwt-key" cfg:"jwt_key" env:"OAUTH2_PROXY_JWT_KEY"`
	JWTKeyFile            string `flag:"jwt-key-file" cfg:"jwt_key_file" env:"OAUTH2_PROXY_JWT_KEY_FILE"`
	PubJWKURL             string `flag:"pubjwk-url" cfg:"pubjwk_url" env:"OAUTH2_PROXY_PUBJWK_URL"`
	GCPHealthChecks       bool   `flag:"gcp-healthchecks" cfg:"gcp_healthchecks" env:"OAUTH2_PROXY_GCP_HEALTHCHECKS"`

	// internal values that are set after config validation
	redirectURL        *url.URL
	proxyURLs          []*url.URL
	compiledRegex      []*regexp.Regexp
	compiledHeader     []*regexp.Regexp
	provider           providers.Provider
	sessionStore       sessionsapi.SessionStore
	signatureData      *SignatureData
	oidcVerifier       *oidc.IDTokenVerifier
	jwtBearerVerifiers []*oidc.IDTokenVerifier
}

// SignatureData holds hmacauth signature hash and key
type SignatureData struct {
	hash crypto.Hash
	key  string
}

// NewOptions constructs a new Options with defaulted values
func NewOptions() *Options {
	return &Options{
		ProxyPrefix:         "/oauth2",
		PingPath:            "/ping",
		ProxyWebSockets:     true,
		HTTPAddress:         "127.0.0.1:4180",
		HTTPSAddress:        ":443",
		ForceHTTPS:          false,
		DisplayHtpasswdForm: true,
		Cookie: options.CookieOptions{
			Name:     "_oauth2_proxy",
			Secure:   true,
			HTTPOnly: true,
			Expire:   time.Duration(168) * time.Hour,
			Refresh:  time.Duration(0),
		},
		Session: options.SessionOptions{
			Type: "cookie",
		},
		SetXAuthRequest:                  false,
		SkipAuthPreflight:                false,
		PassBasicAuth:                    true,
		SetBasicAuth:                     false,
		PassUserHeaders:                  true,
		PassAccessToken:                  false,
		PassHostHeader:                   true,
		SetAuthorization:                 false,
		PassAuthorization:                false,
		PreferEmailToUser:                false,
		Prompt:                           "", // Change to "login" when ApprovalPrompt officially deprecated
		ApprovalPrompt:                   "force",
		UserIDClaim:                      "email",
		InsecureOIDCAllowUnverifiedEmail: false,
		SkipOIDCDiscovery:                false,
		LoggingFilename:                  "",
		LoggingMaxSize:                   100,
		LoggingMaxAge:                    7,
		LoggingMaxBackups:                0,
		LoggingLocalTime:                 true,
		LoggingCompress:                  false,
		ExcludeLoggingPaths:              "",
		SilencePingLogging:               false,
		StandardLogging:                  true,
		StandardLoggingFormat:            logger.DefaultStandardLoggingFormat,
		RequestLogging:                   true,
		RequestLoggingFormat:             logger.DefaultRequestLoggingFormat,
		AuthLogging:                      true,
		AuthLoggingFormat:                logger.DefaultAuthLoggingFormat,
	}
}

// jwtIssuer hold parsed JWT issuer info that's used to construct a verifier.
type jwtIssuer struct {
	issuerURI string
	audience  string
}

func parseURL(toParse string, urltype string, msgs []string) (*url.URL, []string) {
	parsed, err := url.Parse(toParse)
	if err != nil {
		return nil, append(msgs, fmt.Sprintf(
			"error parsing %s-url=%q %s", urltype, toParse, err))
	}
	return parsed, msgs
}

// Validate checks that required options are set and validates those that they
// are of the correct format
func (o *Options) Validate() error {
	if o.SSLInsecureSkipVerify {
		// TODO: Accept a certificate bundle.
		insecureTransport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		http.DefaultClient = &http.Client{Transport: insecureTransport}
	}

	msgs := make([]string, 0)
	if o.Cookie.Secret == "" {
		msgs = append(msgs, "missing setting: cookie-secret")
	}
	if o.ClientID == "" {
		msgs = append(msgs, "missing setting: client-id")
	}
	// login.gov uses a signed JWT to authenticate, not a client-secret
	if o.Provider != "login.gov" {
		if o.ClientSecret == "" && o.ClientSecretFile == "" {
			msgs = append(msgs, "missing setting: client-secret or client-secret-file")
		}
		if o.ClientSecret == "" && o.ClientSecretFile != "" {
			_, err := ioutil.ReadFile(o.ClientSecretFile)
			if err != nil {
				msgs = append(msgs, "could not read client secret file: "+o.ClientSecretFile)
			}
		}
	}
	if o.AuthenticatedEmailsFile == "" && len(o.EmailDomains) == 0 && o.HtpasswdFile == "" {
		msgs = append(msgs, "missing setting for email validation: email-domain or authenticated-emails-file required."+
			"\n      use email-domain=* to authorize all email addresses")
	}

	if o.SetBasicAuth && o.SetAuthorization {
		msgs = append(msgs, "mutually exclusive: set-basic-auth and set-authorization-header can not both be true")
	}

	if o.OIDCIssuerURL != "" {

		ctx := context.Background()

		if o.InsecureOIDCSkipIssuerVerification && !o.SkipOIDCDiscovery {
			// go-oidc doesn't let us pass bypass the issuer check this in the oidc.NewProvider call
			// (which uses discovery to get the URLs), so we'll do a quick check ourselves and if
			// we get the URLs, we'll just use the non-discovery path.

			logger.Printf("Performing OIDC Discovery...")

			if req, err := http.NewRequest("GET", strings.TrimSuffix(o.OIDCIssuerURL, "/")+"/.well-known/openid-configuration", nil); err == nil {
				if body, err := requests.Request(req); err == nil {

					// Prefer manually configured URLs. It's a bit unclear
					// why you'd be doing discovery and also providing the URLs
					// explicitly though...
					if o.LoginURL == "" {
						o.LoginURL = body.Get("authorization_endpoint").MustString()
					}

					if o.RedeemURL == "" {
						o.RedeemURL = body.Get("token_endpoint").MustString()
					}

					if o.OIDCJwksURL == "" {
						o.OIDCJwksURL = body.Get("jwks_uri").MustString()
					}

					if o.ProfileURL == "" {
						o.ProfileURL = body.Get("userinfo_endpoint").MustString()
					}

					o.SkipOIDCDiscovery = true
				} else {
					logger.Printf("error: failed to discover OIDC configuration: %v", err)
				}
			} else {
				logger.Printf("error: failed parsing OIDC discovery URL: %v", err)
			}
		}

		// Construct a manual IDTokenVerifier from issuer URL & JWKS URI
		// instead of metadata discovery if we enable -skip-oidc-discovery.
		// In this case we need to make sure the required endpoints for
		// the provider are configured.
		if o.SkipOIDCDiscovery {
			if o.LoginURL == "" {
				msgs = append(msgs, "missing setting: login-url")
			}
			if o.RedeemURL == "" {
				msgs = append(msgs, "missing setting: redeem-url")
			}
			if o.OIDCJwksURL == "" {
				msgs = append(msgs, "missing setting: oidc-jwks-url")
			}
			keySet := oidc.NewRemoteKeySet(ctx, o.OIDCJwksURL)
			o.oidcVerifier = oidc.NewVerifier(o.OIDCIssuerURL, keySet, &oidc.Config{
				ClientID:        o.ClientID,
				SkipIssuerCheck: o.InsecureOIDCSkipIssuerVerification,
			})
		} else {
			// Configure discoverable provider data.
			provider, err := oidc.NewProvider(ctx, o.OIDCIssuerURL)
			if err != nil {
				return err
			}
			o.oidcVerifier = provider.Verifier(&oidc.Config{
				ClientID:        o.ClientID,
				SkipIssuerCheck: o.InsecureOIDCSkipIssuerVerification,
			})

			o.LoginURL = provider.Endpoint().AuthURL
			o.RedeemURL = provider.Endpoint().TokenURL
		}
		if o.Scope == "" {
			o.Scope = "openid email profile"
		}
	}

	if o.PreferEmailToUser && !o.PassBasicAuth && !o.PassUserHeaders {
		msgs = append(msgs, "PreferEmailToUser should only be used with PassBasicAuth or PassUserHeaders")
	}

	if o.SkipJwtBearerTokens {
		// If we are using an oidc provider, go ahead and add that provider to the list
		if o.oidcVerifier != nil {
			o.jwtBearerVerifiers = append(o.jwtBearerVerifiers, o.oidcVerifier)
		}
		// Configure extra issuers
		if len(o.ExtraJwtIssuers) > 0 {
			var jwtIssuers []jwtIssuer
			jwtIssuers, msgs = parseJwtIssuers(o.ExtraJwtIssuers, msgs)
			for _, jwtIssuer := range jwtIssuers {
				verifier, err := newVerifierFromJwtIssuer(jwtIssuer)
				if err != nil {
					msgs = append(msgs, fmt.Sprintf("error building verifiers: %s", err))
				}
				o.jwtBearerVerifiers = append(o.jwtBearerVerifiers, verifier)
			}
		}
	}

	o.redirectURL, msgs = parseURL(o.RedirectURL, "redirect", msgs)

	for _, u := range o.Upstreams {
		upstreamURL, err := url.Parse(u)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("error parsing upstream: %s", err))
		} else {
			if upstreamURL.Path == "" {
				upstreamURL.Path = "/"
			}
			o.proxyURLs = append(o.proxyURLs, upstreamURL)
		}
	}

	for _, u := range o.SkipAuthRegex {
		compiledRegex, err := regexp.Compile(u)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("error compiling regex=%q %s", u, err))
			continue
		}
		o.compiledRegex = append(o.compiledRegex, compiledRegex)
	}

	for _, u := range o.SkipAuthHeader {
		compiledHeader, err := regexp.Compile(u)
		if err != nil {
			msgs = append(msgs, fmt.Sprintf("error compiling regex=%q %s", u, err))
			continue
		}
		o.compiledHeader = append(o.compiledHeader, compiledHeader)
	}

	msgs = parseProviderInfo(o, msgs)

	var cipher *encryption.Cipher
	if o.PassAccessToken || o.SetAuthorization || o.PassAuthorization || (o.Cookie.Refresh != time.Duration(0)) {
		validCookieSecretSize := false
		for _, i := range []int{16, 24, 32} {
			if len(secretBytes(o.Cookie.Secret)) == i {
				validCookieSecretSize = true
			}
		}
		var decoded bool
		if string(secretBytes(o.Cookie.Secret)) != o.Cookie.Secret {
			decoded = true
		}
		if !validCookieSecretSize {
			var suffix string
			if decoded {
				suffix = fmt.Sprintf(" note: cookie secret was base64 decoded from %q", o.Cookie.Secret)
			}
			msgs = append(msgs, fmt.Sprintf(
				"cookie_secret must be 16, 24, or 32 bytes "+
					"to create an AES cipher when "+
					"pass_access_token == true or "+
					"cookie_refresh != 0, but is %d bytes.%s",
				len(secretBytes(o.Cookie.Secret)), suffix))
		} else {
			var err error
			cipher, err = encryption.NewCipher(secretBytes(o.Cookie.Secret))
			if err != nil {
				msgs = append(msgs, fmt.Sprintf("cookie-secret error: %v", err))
			}
		}
	}

	o.Session.Cipher = cipher
	sessionStore, err := sessions.NewSessionStore(&o.Session, &o.Cookie)
	if err != nil {
		msgs = append(msgs, fmt.Sprintf("error initialising session storage: %v", err))
	} else {
		o.sessionStore = sessionStore
	}

	if o.Cookie.Refresh >= o.Cookie.Expire {
		msgs = append(msgs, fmt.Sprintf(
			"cookie_refresh (%s) must be less than "+
				"cookie_expire (%s)",
			o.Cookie.Refresh.String(),
			o.Cookie.Expire.String()))
	}

	if len(o.GoogleGroups) > 0 || o.GoogleAdminEmail != "" || o.GoogleServiceAccountJSON != "" {
		if len(o.GoogleGroups) < 1 {
			msgs = append(msgs, "missing setting: google-group")
		}
		if o.GoogleAdminEmail == "" {
			msgs = append(msgs, "missing setting: google-admin-email")
		}
		if o.GoogleServiceAccountJSON == "" {
			msgs = append(msgs, "missing setting: google-service-account-json")
		}
	}

	switch o.Cookie.SameSite {
	case "", "none", "lax", "strict":
	default:
		msgs = append(msgs, fmt.Sprintf("cookie_samesite (%s) must be one of ['', 'lax', 'strict', 'none']", o.Cookie.SameSite))
	}

	// Sort cookie domains by length, so that we try longer (and more specific)
	// domains first
	sort.Slice(o.Cookie.Domains, func(i, j int) bool {
		return len(o.Cookie.Domains[i]) > len(o.Cookie.Domains[j])
	})

	msgs = parseSignatureKey(o, msgs)
	msgs = validateCookieName(o, msgs)
	msgs = setupLogger(o, msgs)

	if len(msgs) != 0 {
		return fmt.Errorf("invalid configuration:\n  %s",
			strings.Join(msgs, "\n  "))
	}
	return nil
}

func parseProviderInfo(o *Options, msgs []string) []string {
	p := &providers.ProviderData{
		Scope:            o.Scope,
		ClientID:         o.ClientID,
		ClientSecret:     o.ClientSecret,
		ClientSecretFile: o.ClientSecretFile,
		Prompt:           o.Prompt,
		ApprovalPrompt:   o.ApprovalPrompt,
		AcrValues:        o.AcrValues,
	}
	p.LoginURL, msgs = parseURL(o.LoginURL, "login", msgs)
	p.RedeemURL, msgs = parseURL(o.RedeemURL, "redeem", msgs)
	p.ProfileURL, msgs = parseURL(o.ProfileURL, "profile", msgs)
	p.ValidateURL, msgs = parseURL(o.ValidateURL, "validate", msgs)
	p.ProtectedResource, msgs = parseURL(o.ProtectedResource, "resource", msgs)

	o.provider = providers.New(o.Provider, p)
	switch p := o.provider.(type) {
	case *providers.AzureProvider:
		p.Configure(o.AzureTenant)
	case *providers.GitHubProvider:
		p.SetOrgTeam(o.GitHubOrg, o.GitHubTeam)
	case *providers.KeycloakProvider:
		p.SetGroup(o.KeycloakGroup)
	case *providers.GoogleProvider:
		if o.GoogleServiceAccountJSON != "" {
			file, err := os.Open(o.GoogleServiceAccountJSON)
			if err != nil {
				msgs = append(msgs, "invalid Google credentials file: "+o.GoogleServiceAccountJSON)
			} else {
				p.SetGroupRestriction(o.GoogleGroups, o.GoogleAdminEmail, file)
			}
		}
	case *providers.BitbucketProvider:
		p.SetTeam(o.BitbucketTeam)
		p.SetRepository(o.BitbucketRepository)
	case *providers.OIDCProvider:
		p.AllowUnverifiedEmail = o.InsecureOIDCAllowUnverifiedEmail
		p.UserIDClaim = o.UserIDClaim
		if o.oidcVerifier == nil {
			msgs = append(msgs, "oidc provider requires an oidc issuer URL")
		} else {
			p.Verifier = o.oidcVerifier
		}
	case *providers.GitLabProvider:
		p.AllowUnverifiedEmail = o.InsecureOIDCAllowUnverifiedEmail
		p.Group = o.GitLabGroup
		p.EmailDomains = o.EmailDomains

		if o.oidcVerifier != nil {
			p.Verifier = o.oidcVerifier
		} else {
			// Initialize with default verifier for gitlab.com
			ctx := context.Background()

			provider, err := oidc.NewProvider(ctx, "https://gitlab.com")
			if err != nil {
				msgs = append(msgs, "failed to initialize oidc provider for gitlab.com")
			} else {
				p.Verifier = provider.Verifier(&oidc.Config{
					ClientID: o.ClientID,
				})

				p.LoginURL, msgs = parseURL(provider.Endpoint().AuthURL, "login", msgs)
				p.RedeemURL, msgs = parseURL(provider.Endpoint().TokenURL, "redeem", msgs)
			}
		}
	case *providers.LoginGovProvider:
		p.PubJWKURL, msgs = parseURL(o.PubJWKURL, "pubjwk", msgs)

		// JWT key can be supplied via env variable or file in the filesystem, but not both.
		switch {
		case o.JWTKey != "" && o.JWTKeyFile != "":
			msgs = append(msgs, "cannot set both jwt-key and jwt-key-file options")
		case o.JWTKey == "" && o.JWTKeyFile == "":
			msgs = append(msgs, "login.gov provider requires a private key for signing JWTs")
		case o.JWTKey != "":
			// The JWT Key is in the commandline argument
			signKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(o.JWTKey))
			if err != nil {
				msgs = append(msgs, "could not parse RSA Private Key PEM")
			} else {
				p.JWTKey = signKey
			}
		case o.JWTKeyFile != "":
			// The JWT key is in the filesystem
			keyData, err := ioutil.ReadFile(o.JWTKeyFile)
			if err != nil {
				msgs = append(msgs, "could not read key file: "+o.JWTKeyFile)
			}
			signKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyData)
			if err != nil {
				msgs = append(msgs, "could not parse private key from PEM file:"+o.JWTKeyFile)
			} else {
				p.JWTKey = signKey
			}
		}
	}
	return msgs
}

func parseSignatureKey(o *Options, msgs []string) []string {
	if o.SignatureKey == "" {
		return msgs
	}

	components := strings.Split(o.SignatureKey, ":")
	if len(components) != 2 {
		return append(msgs, "invalid signature hash:key spec: "+
			o.SignatureKey)
	}

	algorithm, secretKey := components[0], components[1]
	var hash crypto.Hash
	var err error
	if hash, err = hmacauth.DigestNameToCryptoHash(algorithm); err != nil {
		return append(msgs, "unsupported signature hash algorithm: "+
			o.SignatureKey)
	}
	o.signatureData = &SignatureData{hash: hash, key: secretKey}
	return msgs
}

// parseJwtIssuers takes in an array of strings in the form of issuer=audience
// and parses to an array of jwtIssuer structs.
func parseJwtIssuers(issuers []string, msgs []string) ([]jwtIssuer, []string) {
	parsedIssuers := make([]jwtIssuer, 0, len(issuers))
	for _, jwtVerifier := range issuers {
		components := strings.Split(jwtVerifier, "=")
		if len(components) < 2 {
			msgs = append(msgs, fmt.Sprintf("invalid jwt verifier uri=audience spec: %s", jwtVerifier))
			continue
		}
		uri, audience := components[0], strings.Join(components[1:], "=")
		parsedIssuers = append(parsedIssuers, jwtIssuer{issuerURI: uri, audience: audience})
	}
	return parsedIssuers, msgs
}

// newVerifierFromJwtIssuer takes in issuer information in jwtIssuer info and returns
// a verifier for that issuer.
func newVerifierFromJwtIssuer(jwtIssuer jwtIssuer) (*oidc.IDTokenVerifier, error) {
	config := &oidc.Config{
		ClientID: jwtIssuer.audience,
	}
	// Try as an OpenID Connect Provider first
	var verifier *oidc.IDTokenVerifier
	provider, err := oidc.NewProvider(context.Background(), jwtIssuer.issuerURI)
	if err != nil {
		// Try as JWKS URI
		jwksURI := strings.TrimSuffix(jwtIssuer.issuerURI, "/") + "/.well-known/jwks.json"
		_, err := http.NewRequest("GET", jwksURI, nil)
		if err != nil {
			return nil, err
		}
		verifier = oidc.NewVerifier(jwtIssuer.issuerURI, oidc.NewRemoteKeySet(context.Background(), jwksURI), config)
	} else {
		verifier = provider.Verifier(config)
	}
	return verifier, nil
}

func validateCookieName(o *Options, msgs []string) []string {
	cookie := &http.Cookie{Name: o.Cookie.Name}
	if cookie.String() == "" {
		return append(msgs, fmt.Sprintf("invalid cookie name: %q", o.Cookie.Name))
	}
	return msgs
}

func addPadding(secret string) string {
	padding := len(secret) % 4
	switch padding {
	case 1:
		return secret + "==="
	case 2:
		return secret + "=="
	case 3:
		return secret + "="
	default:
		return secret
	}
}

// secretBytes attempts to base64 decode the secret, if that fails it treats the secret as binary
func secretBytes(secret string) []byte {
	b, err := base64.URLEncoding.DecodeString(addPadding(secret))
	if err == nil {
		return []byte(addPadding(string(b)))
	}
	return []byte(secret)
}

func setupLogger(o *Options, msgs []string) []string {
	// Setup the log file
	if len(o.LoggingFilename) > 0 {
		// Validate that the file/dir can be written
		file, err := os.OpenFile(o.LoggingFilename, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			if os.IsPermission(err) {
				return append(msgs, "unable to write to log file: "+o.LoggingFilename)
			}
		}
		file.Close()

		logger.Printf("Redirecting logging to file: %s", o.LoggingFilename)

		logWriter := &lumberjack.Logger{
			Filename:   o.LoggingFilename,
			MaxSize:    o.LoggingMaxSize, // megabytes
			MaxAge:     o.LoggingMaxAge,  // days
			MaxBackups: o.LoggingMaxBackups,
			LocalTime:  o.LoggingLocalTime,
			Compress:   o.LoggingCompress,
		}

		logger.SetOutput(logWriter)
	}

	// Supply a sanity warning to the logger if all logging is disabled
	if !o.StandardLogging && !o.AuthLogging && !o.RequestLogging {
		logger.Print("Warning: Logging disabled. No further logs will be shown.")
	}

	// Pass configuration values to the standard logger
	logger.SetStandardEnabled(o.StandardLogging)
	logger.SetAuthEnabled(o.AuthLogging)
	logger.SetReqEnabled(o.RequestLogging)
	logger.SetStandardTemplate(o.StandardLoggingFormat)
	logger.SetAuthTemplate(o.AuthLoggingFormat)
	logger.SetReqTemplate(o.RequestLoggingFormat)
	logger.SetReverseProxy(o.ReverseProxy)

	excludePaths := make([]string, 0)
	excludePaths = append(excludePaths, strings.Split(o.ExcludeLoggingPaths, ",")...)
	if o.SilencePingLogging {
		excludePaths = append(excludePaths, o.PingPath)
	}

	logger.SetExcludePaths(excludePaths)

	if !o.LoggingLocalTime {
		logger.SetFlags(logger.Flags() | logger.LUTC)
	}

	return msgs
}
