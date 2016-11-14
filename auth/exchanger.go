// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"time"

	"github.com/TheThingsNetwork/go-account-lib/account"
	"github.com/TheThingsNetwork/go-account-lib/auth"
	"github.com/TheThingsNetwork/go-account-lib/util"
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
	var t account.Token
	err = util.GET(a.ctx, a.accountServer, auth.AccessKey(key), fmt.Sprintf("/api/v2/gateways/%s/token", gatewayID), &t)
	if err != nil {
		return
	}
	return t.Token, t.Expires, nil
}
