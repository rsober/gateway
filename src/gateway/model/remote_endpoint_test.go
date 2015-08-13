package model_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx/types"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"gateway/db"
	"gateway/db/mongo"
	pq "gateway/db/postgres"
	sqls "gateway/db/sqlserver"
	"gateway/model"
	re "gateway/model/remote_endpoint"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { gc.TestingT(t) }

type RemoteEndpointSuite struct{}

var _ = gc.Suite(&RemoteEndpointSuite{})

func data() map[string]interface{} {
	return map[string]interface{}{
		"sqls-simple": map[string]interface{}{
			"config": map[string]interface{}{
				"server":   "some.url.net",
				"port":     1234,
				"user id":  "user",
				"password": "pass",
				"database": "db",
				"schema":   "dbschema",
			},
		},
		"sqls-complicated": map[string]interface{}{
			"config": map[string]interface{}{
				"server":             "some.url.net",
				"port":               1234,
				"user id":            "user",
				"password":           "pass",
				"database":           "db",
				"schema":             "dbschema",
				"connection timeout": 30,
			},
			"maxOpenConn": 80,
			"maxIdleConn": 100,
		},
		"sqls-badConfig": map[string]interface{}{
			"config": map[string]interface{}{
				"server": "some.url.net",
			},
		},
		"sqls-badConfigType": map[string]interface{}{
			"config": 8,
		},
		"sqls-badMaxIdleType": map[string]interface{}{
			"config": map[string]interface{}{
				"server":   "some.url.net",
				"port":     1234,
				"user id":  "user",
				"password": "pass",
				"database": "db",
				"schema":   "dbschema",
			},
			"maxOpenConn": "hello",
		},
		"pq-simple": map[string]interface{}{
			"config": map[string]interface{}{
				"host":     "some.url.net",
				"port":     1234,
				"user":     "user",
				"password": "pass",
				"dbname":   "db",
			},
		},
		"pq-complicated": map[string]interface{}{
			"config": map[string]interface{}{
				"host":            "some.url.net",
				"port":            1234,
				"user":            "user",
				"password":        "pass",
				"dbname":          "db",
				"connect_timeout": 30,
			},
			"maxOpenConn": 80,
			"maxIdleConn": 100,
		},
		"pq-badConfig": map[string]interface{}{
			"config": map[string]interface{}{
				"host": "some.url.net",
			},
		},
		"pq-badConfigType": map[string]interface{}{
			"config": 8,
		},
		"pq-badMaxIdleType": map[string]interface{}{
			"config": map[string]interface{}{
				"host":     "some.url.net",
				"port":     1234,
				"user":     "user",
				"password": "pass",
				"dbname":   "db",
			},
			"maxOpenConn": "hello",
		},
		"mongo-complicated": map[string]interface{}{
			"config": map[string]interface{}{
				"hosts": []interface{}{
					map[string]interface{}{
						"host": "test.com",
						"port": float64(123),
					},
				},
				"username": "user",
				"password": "pass",
				"database": "db",
			},
			"limit": 123,
		},
	}
}

func specs() map[string]db.Specifier {
	specs := make(map[string]db.Specifier)
	for _, which := range []struct {
		name string
		kind string
	}{{
		"sqls-simple", model.RemoteEndpointTypeSQLServer,
	}, {
		"sqls-complicated", model.RemoteEndpointTypeSQLServer,
	}, {
		"pq-simple", model.RemoteEndpointTypePostgres,
	}, {
		"pq-complicated", model.RemoteEndpointTypePostgres,
	}, {
		"mongo-complicated", model.RemoteEndpointTypeMongo,
	}} {
		d := data()[which.name].(map[string]interface{})
		js, err := json.Marshal(d)
		if err != nil {
			panic(err)
		}
		var s db.Specifier
		switch which.kind {
		case model.RemoteEndpointTypeSQLServer:
			var conf re.SQLServer
			err = json.Unmarshal(js, &conf)
			if err != nil {
				panic(err)
			}
			s, err = sqls.Config(
				sqls.Connection(conf.Config),
				sqls.MaxOpenIdle(conf.MaxOpenConn, conf.MaxIdleConn),
			)
		case model.RemoteEndpointTypePostgres:
			var conf re.Postgres
			err = json.Unmarshal(js, &conf)
			if err != nil {
				panic(err)
			}
			s, err = pq.Config(
				pq.Connection(conf.Config),
				pq.MaxOpenIdle(conf.MaxOpenConn, conf.MaxIdleConn),
			)
		case model.RemoteEndpointTypeMongo:
			var conf re.Mongo
			err = json.Unmarshal(js, &conf)
			if err != nil {
				panic(err)
			}
			s, err = mongo.Config(
				mongo.Connection(conf.Config),
				mongo.PoolLimit(conf.Limit),
			)
		default:
			err = fmt.Errorf("no such type %q", which.kind)
		}
		if err != nil {
			panic(err)
		}
		specs[which.name] = s
	}
	return specs
}

func (s *RemoteEndpointSuite) TestDBConfig(c *gc.C) {
	for i, t := range []struct {
		should      string
		givenConfig string
		givenType   string
		expectSpec  string
		expectError string
	}{{
		should:      "(SQL) work with a simple config",
		givenConfig: "sqls-simple",
		givenType:   model.RemoteEndpointTypeSQLServer,
		expectSpec:  "sqls-simple",
	}, {
		should:      "(SQL) work with a complex config",
		givenConfig: "sqls-complicated",
		givenType:   model.RemoteEndpointTypeSQLServer,
		expectSpec:  "sqls-complicated",
	}, {
		should:      "(SQL) fail with a bad config",
		givenConfig: "sqls-badConfig",
		givenType:   model.RemoteEndpointTypeSQLServer,
		expectError: `SQL Config missing "port" key`,
	}, {
		should:      "(SQL) fail with a bad config type",
		givenConfig: "sqls-badConfigType",
		givenType:   model.RemoteEndpointTypeSQLServer,
		expectError: `bad JSON for SQL Server config: json: cannot unmarshal number into Go value of type sqlserver.Conn`,
	}, {
		should:      "(SQL) fail with a bad max idle type",
		givenConfig: "sqls-badMaxIdleType",
		givenType:   model.RemoteEndpointTypeSQLServer,
		expectError: `bad JSON for SQL Server config: json: cannot unmarshal string into Go value of type int`,
	}, {
		should:      "(PSQL) work with a simple config",
		givenConfig: "pq-simple",
		givenType:   model.RemoteEndpointTypePostgres,
		expectSpec:  "pq-simple",
	}, {
		should:      "(PSQL) work with a complex config",
		givenConfig: "pq-complicated",
		givenType:   model.RemoteEndpointTypePostgres,
		expectSpec:  "pq-complicated",
	}, {
		should:      "(PSQL) fail with a bad config",
		givenConfig: "pq-badConfig",
		givenType:   model.RemoteEndpointTypePostgres,
		expectError: `Postgres Config missing "port" key`,
	}, {
		should:      "(PSQL) fail with a bad config type",
		givenConfig: "pq-badConfigType",
		givenType:   model.RemoteEndpointTypePostgres,
		expectError: `bad JSON for Postgres config: json: cannot unmarshal number into Go value of type postgres.Conn`,
	}, {
		should:      "(PSQL) fail with a bad max idle type",
		givenConfig: "pq-badMaxIdleType",
		givenType:   model.RemoteEndpointTypePostgres,
		expectError: `bad JSON for Postgres config: json: cannot unmarshal string into Go value of type int`,
	}, {
		should:      "Mongo work with a complex config",
		givenConfig: "mongo-complicated",
		givenType:   model.RemoteEndpointTypeMongo,
		expectSpec:  "mongo-complicated",
	}} {
		c.Logf("Test %d: should %s", i, t.should)
		data := data()[t.givenConfig]
		dataJSON, err := json.Marshal(data)
		endpoint := &model.RemoteEndpoint{
			Type: t.givenType,
			Data: types.JsonText(json.RawMessage(dataJSON)),
		}
		spec, err := endpoint.DBConfig()
		if t.expectError != "" {
			c.Check(err, gc.ErrorMatches, t.expectError)
			continue
		}
		c.Assert(err, jc.ErrorIsNil)
		expectSpec := specs()[t.expectSpec]
		c.Check(spec, jc.DeepEquals, expectSpec)
	}
}
