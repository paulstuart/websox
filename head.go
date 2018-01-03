// Copyright 2017 Paul Stuart
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websox provides a wrapper for a client to initiate and handle
// data push requests from a server
package websox

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Oauth2Header returns headers to access protected http endpoints
func Oauth2Header(ctx context.Context, url, cid, secret string) (http.Header, error) {
	tokenSource := MakeClientCredentialsTokenSource(ctx, url, cid, secret)
	token, err := tokenSource.Token()
	if err != nil {
		return nil, errors.Wrap(err, "error getting token")
	}

	header := make(http.Header)
	header.Add("Authorization", "Bearer "+token.AccessToken)
	return header, nil
}

// MakeClientCredentialsTokenSource returns an oauth2 token source for validation
func MakeClientCredentialsTokenSource(ctx context.Context, url, cid, secret string) oauth2.TokenSource {
	conf := &clientcredentials.Config{
		ClientID:     cid,
		ClientSecret: secret,
		TokenURL:     url,
	}
	return conf.TokenSource(ctx)
}
