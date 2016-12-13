// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package auth

import (
	"time"

	"github.com/TheThingsNetwork/go-account-lib/account"
	"github.com/apex/log"
)

// Exchanger interface exchanges a gateway access key for a gateway token
type Exchanger interface {
	Exchange(gatewayID, key string) (token string, expires time.Time, err error)
}

// NewAccountServer returns a new AccountServerExchanger for the given TTN Account Server
func NewAccountServer(accountServer string, ctx log.Interface) Exchanger {
	return &AccountServerExchanger{
		accountServer: accountServer,
		ctx:           ctx,
	}
}

// AccountServerExchanger uses a TTN Account Server to exchange the key for a token
type AccountServerExchanger struct {
	ctx           log.Interface
	accountServer string
}

// Exchange implements the Exchanger interface
func (a *AccountServerExchanger) Exchange(gatewayID, key string) (token string, expires time.Time, err error) {
	acct := account.NewWithKey(a.accountServer, key)
	t, err := acct.GetGatewayToken(gatewayID)
	if err != nil {
		return token, expires, err
	}
	return t.AccessToken, t.Expiry, nil
}
