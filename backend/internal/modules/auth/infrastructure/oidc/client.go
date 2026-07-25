package oidc

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	"github.com/duclamdev/application-chat/backend/internal/shared/outboundhttp"
	"golang.org/x/oauth2"
)

type Client struct {
	httpClient *http.Client
}

type discoveryMetadata struct {
	JWKSURL string `json:"jwks_uri"`
}

func NewClient() *Client {
	return &Client{httpClient: outboundhttp.NewPublicClient(15*time.Second, true)}
}

func (c *Client) AuthorizationURL(
	ctx context.Context,
	providerConfig tenancydomain.OIDCProvider,
	clientSecret string,
	redirectURI string,
	state string,
	nonce string,
	codeVerifier string,
) (string, error) {
	provider, oauthConfig, err := c.provider(ctx, providerConfig, clientSecret, redirectURI)
	if err != nil {
		return "", err
	}
	_ = provider
	return oauthConfig.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(codeVerifier),
	), nil
}

func (c *Client) ExchangeAndVerify(
	ctx context.Context,
	providerConfig tenancydomain.OIDCProvider,
	clientSecret string,
	redirectURI string,
	code string,
	codeVerifier string,
	nonce string,
) (map[string]any, error) {
	provider, oauthConfig, err := c.provider(ctx, providerConfig, clientSecret, redirectURI)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	token, err := oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return nil, errors.New("oidc token response is missing id_token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: providerConfig.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return nil, errors.New("oidc nonce mismatch")
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (c *Client) provider(
	ctx context.Context,
	providerConfig tenancydomain.OIDCProvider,
	clientSecret string,
	redirectURI string,
) (*oidc.Provider, oauth2.Config, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	provider, err := oidc.NewProvider(ctx, providerConfig.IssuerURL)
	if err != nil {
		return nil, oauth2.Config{}, err
	}
	endpoint := provider.Endpoint()
	var metadata discoveryMetadata
	if err := provider.Claims(&metadata); err != nil {
		return nil, oauth2.Config{}, err
	}
	for _, rawURL := range []string{endpoint.AuthURL, endpoint.TokenURL, metadata.JWKSURL} {
		parsed, parseErr := url.Parse(rawURL)
		if parseErr != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
			return nil, oauth2.Config{}, errors.New("oidc discovery returned an unsafe endpoint")
		}
	}
	return provider, oauth2.Config{
		ClientID:     providerConfig.ClientID,
		ClientSecret: clientSecret,
		Endpoint:     endpoint,
		RedirectURL:  redirectURI,
		Scopes:       append([]string{}, providerConfig.Scopes...),
	}, nil
}

var _ authapp.OIDCProtocol = (*Client)(nil)
