// https://github.com/topfreegames/mystack-controller
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package extensions

import (
	"time"

	"github.com/topfreegames/mystack-controller/errors"
	"github.com/topfreegames/mystack-controller/models"
	"golang.org/x/oauth2"
)

//SaveToken writes the token parameters on DB
func SaveToken(token *oauth2.Token, email, keyAccessToken string, db models.DB) error {
	query := `INSERT INTO users(access_token, refresh_token, expiry, token_type, email, key_access_token) 
	VALUES(:access_token, :refresh_token, :expiry, :token_type, :email, :key_access_token)
	ON CONFLICT(email) DO UPDATE
		SET access_token = excluded.access_token,
				refresh_token = excluded.refresh_token,
				expiry = excluded.expiry;`

	if token.RefreshToken == "" {
		query = `UPDATE users 
		SET access_token = :access_token,
				expiry = :expiry,
				key_access_token = :key_access_token
		WHERE email = :email
		`
	}

	values := map[string]interface{}{
		"access_token":     token.AccessToken,
		"refresh_token":    token.RefreshToken,
		"expiry":           token.Expiry,
		"token_type":       token.TokenType,
		"email":            email,
		"key_access_token": keyAccessToken,
	}
	_, err := db.NamedExec(query, values)

	if err != nil {
		return errors.NewDatabaseError(err)
	}

	return nil
}

//Token reads token from DB
func Token(accessToken string, db models.DB) (*oauth2.Token, error) {
	query := `SELECT access_token, refresh_token, expiry, token_type
						FROM users
						WHERE key_access_token = $1`

	destToken := struct {
		AccessToken  string    `db:"access_token"`
		RefreshToken string    `db:"refresh_token"`
		Expiry       time.Time `db:"expiry"`
		TokenType    string    `db:"token_type"`
	}{}

	err := db.Get(&destToken, query, accessToken)
	if err != nil {
		return nil, errors.NewAccessError("Access Token not found (have you logged in?)", err)
	}

	token := &oauth2.Token{
		AccessToken:  destToken.AccessToken,
		RefreshToken: destToken.RefreshToken,
		Expiry:       destToken.Expiry,
		TokenType:    destToken.TokenType,
	}

	return token, nil
}
