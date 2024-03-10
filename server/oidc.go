package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
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
	oidc *oidc.IDTokenVerifier
}

func NewVerifier(ctx context.Context, issuerURL string, jwksURL string) (*Verifier, error) {
	ks := oidc.NewRemoteKeySet(ctx, jwksURL)
	return &Verifier{
		oidc: oidc.NewVerifier(issuerURL, ks, &oidc.Config{ClientID: "https://kubernetes.default.svc.cluster.local"}),
	}, nil
}

func (v *Verifier) GetClaims(ctx context.Context, req *http.Request) (*Claims, error) {
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
