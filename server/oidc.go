package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	jwksURL = "https://kubernetes.default.svc/openid/v1/jwks"
)

type Claims struct {
	Iss        string            `json:"iss"`
	Sub        string            `json:"sub"`
	Kubernetes *KubernetesClaims `json:"kubernetes.io"`
}

type KubernetesClaims struct {
	Namespace      string
	Pod            *PodClaims            `json:"pod"`
	ServiceAccount *ServiceAccountClaims `json:"serviceaccount"`
}

type PodClaims struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type ServiceAccountClaims struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type Verifier struct {
	client *http.Client
	oidc   *oidc.IDTokenVerifier
	issuer string
}

func NewVerifier(ctx context.Context, client *http.Client, issuerURL string) (*Verifier, error) {
	ks := oidc.NewRemoteKeySet(ctx, jwksURL)
	return &Verifier{
		client: client,
		oidc:   oidc.NewVerifier(issuerURL, ks, &oidc.Config{ClientID: issuerURL}),
		issuer: issuerURL,
	}, nil
}

func NewVerifierFromToken(ctx context.Context, client *http.Client, ts oauth2.TokenSource) (*Verifier, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	ks := oidc.NewRemoteKeySet(ctx, jwksURL)
	issuerURL, err := getIssuerInsecure(ctx, ts)
	if err != nil {
		return nil, err
	}
	return &Verifier{
		client: client,
		oidc:   oidc.NewVerifier(issuerURL, ks, &oidc.Config{ClientID: issuerURL}),
		issuer: issuerURL,
	}, nil
}

func (v *Verifier) GetClaims(ctx context.Context, req *http.Request) (*Claims, error) {
	// Add the client to the context so oidc library can use it.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, v.client)

	raw := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	token, err := v.oidc.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("error verifying jwt: %w", err)
	}
	claims := new(Claims)
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("error extracting claims: %w", err)
	}

	return claims, nil
}

// getIssuerInsecure is a helper function to extract the issuer from a token source.
// This is insecure in the fact that typically you want to provide the expected issuer of the token
// out of band, but in levias's case, we're using the issuer to match with other incoming tokens,
// so this is being used as a way automatically set configuration.
func getIssuerInsecure(ctx context.Context, ts oauth2.TokenSource) (string, error) {
	token, err := ts.Token()
	if err != nil {
		return "", err
	}

	v := oidc.NewVerifier("", oidc.NewRemoteKeySet(ctx, jwksURL), &oidc.Config{
		// Skip checks - we're only looking to extract the issuer.
		SkipClientIDCheck: true,
		SkipIssuerCheck:   true,
	})

	t, err := v.Verify(ctx, token.AccessToken)
	if err != nil {
		return "", err
	}
	return t.Issuer, nil
}
