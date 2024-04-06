package main

import (
	"context"
	"fmt"
	"net/http"
)

type AuthMiddleware struct {
	verifier *Verifier
}

func (m *AuthMiddleware) WrapHandler(handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {

		claims, err := m.verifier.GetClaims(r.Context(), r)
		if err != nil {
			http.Error(w, fmt.Sprint(err), http.StatusUnauthorized)
			return err
		}
		ns := claims.Kubernetes.Namespace
		pod := claims.Kubernetes.Pod.Name

		ctx = context.WithValue(ctx, namespaceKey{}, ns)
		ctx = context.WithValue(ctx, podKey{}, pod)
		return handler(ctx, w, r, vars)

		/*
				ctx = context.WithValue(ctx, namespaceKey{}, "foo")
				ctx = context.WithValue(ctx, podKey{}, "bar")

			return handler(ctx, w, r, vars)
		*/

	}
}

type namespaceKey struct{}
type podKey struct{}

func GetNamespace(ctx context.Context) string {
	return ctx.Value(namespaceKey{}).(string)
}

func GetPod(ctx context.Context) string {
	return ctx.Value(podKey{}).(string)
}
