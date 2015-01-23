package model

import (
	"gateway/config"
	apsql "gateway/sql"
	"log"
)

type ProxyEndpointCall struct {
	ID                    int64                          `json:"id"`
	ComponentID           int64                          `json:"-" db:"component_id"`
	RemoteEndpointID      int64                          `json:"remote_endpoint_id" db:"remote_endpoint_id"`
	EndpointNameOverride  string                         `json:"endpoint_name_override" db:"endpoint_name_override"`
	Conditional           string                         `json:"conditional"`
	ConditionalPositive   bool                           `json:"conditional_positive" db:"conditional_positive"`
	Position              int64                          `json:"-"`
	BeforeTransformations []*ProxyEndpointTransformation `json:"before,omitempty"`
	AfterTransformations  []*ProxyEndpointTransformation `json:"after,omitempty"`
}

// AllProxyEndpointCallsForEndpointID returns all calls of a set of endpoint component.
func AllProxyEndpointCallsForComponentIDs(db *apsql.DB, componentIDs []int64) ([]*ProxyEndpointCall, error) {
	calls := []*ProxyEndpointCall{}
	numIDs := len(componentIDs)
	if numIDs == 0 {
		return calls, nil
	}

	var ids []interface{}
	for _, id := range componentIDs {
		ids = append(ids, id)
	}

	err := db.Select(&calls,
		"SELECT "+
			"  `id`, `component_id`, `remote_endpoint_id`, "+
			"`endpoint_name_override`, `conditional`, `conditional_positive` "+
			"FROM `proxy_endpoint_calls` "+
			"WHERE `component_id` IN ("+apsql.NQs(numIDs)+") "+
			"ORDER BY `position` ASC;",
		ids...)
	return calls, err
}

// Insert inserts the call into the database as a new row.
func (c *ProxyEndpointCall) Insert(tx *apsql.Tx, componentID, apiID int64,
	position int) error {
	result, err := tx.Exec(
		"INSERT INTO `proxy_endpoint_calls` "+
			"(`component_id`, `remote_endpoint_id`, `endpoint_name_override`, "+
			" `conditional`, `conditional_positive`, `position`) "+
			"VALUES (?, "+
			"  (SELECT `id` FROM `remote_endpoints` WHERE `id` = ? AND `api_id` = ?), "+
			"  ?, ?, ?, ?);",
		componentID, c.RemoteEndpointID, apiID, c.EndpointNameOverride,
		c.Conditional, c.ConditionalPositive, position)
	if err != nil {
		return err
	}
	c.ID, err = result.LastInsertId()
	if err != nil {
		log.Printf("%s Error getting last insert ID for proxy endpoint component: %v",
			config.System, err)
		return err
	}

	for position, transform := range c.BeforeTransformations {
		err = transform.InsertForCall(tx, c.ID, true, position)
		if err != nil {
			return err
		}
	}
	for position, transform := range c.AfterTransformations {
		err = transform.InsertForCall(tx, c.ID, false, position)
		if err != nil {
			return err
		}
	}

	return nil
}
