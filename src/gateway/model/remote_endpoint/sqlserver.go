package remote_endpoint

import (
	"encoding/json"
	"fmt"

	"gateway/db"
	sqls "gateway/db/sqlserver"

	"github.com/jmoiron/sqlx/types"
)

type SQLServer struct {
	Config      sqls.Conn `json:"config"`
	Tx          bool      `json:"transactions"`
	MaxOpenConn int       `json:"maxOpenConn,omitempty"`
	MaxIdleConn int       `json:"maxIdleConn,omitempty"`
}

// SQLServerConfig gets a "gateway/db/sqlserver" Config and returns any errors.
func SQLServerConfig(data types.JsonText) (db.Specifier, error) {
	var conf SQLServer
	err := json.Unmarshal(data, &conf)
	if err != nil {
		return nil, fmt.Errorf("bad JSON for SQL Server config: %s", err.Error())
	}

	spec, err := sqls.Config(
		sqls.Connection(conf.Config),
		sqls.MaxOpenIdle(conf.MaxOpenConn, conf.MaxIdleConn),
	)
	if err != nil {
		return nil, err
	}
	return spec, nil
}
