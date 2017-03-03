package model

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	aperrors "gateway/errors"
	apsql "gateway/sql"
)

// Host represents a host the API is available on.
type Host struct {
	AccountID int64 `json:"-" db:"account_id"`
	UserID    int64 `json:"-"`
	APIID     int64 `json:"api_id" db:"api_id"`

	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`

	Cert       apsql.NullString `json:"cert" db:"cert"`
	PrivateKey apsql.NullString `json:"private_key" db:"private_key"`
	ForceSSL   bool             `json:"force_ssl" db:"force_ssl"`
}

func (h *Host) CertContents() string {
	return h.Cert.String
}

func (h *Host) PrivateKeyContents() string {
	return h.PrivateKey.String
}

func (h *Host) SetCertContents(val string) {
	h.Cert.Scan(val)
}

func (h *Host) SetPrivateKeyContents(val string) {
	h.PrivateKey.Scan(val)
}

func (h *Host) MarshalJSON() ([]byte, error) {
	// custom MarshalJSON to exclude Cert and PrivateKey from
	// being included in the JSON
	temp := &struct {
		APIID    int64  `json:"api_id"`
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Hostname string `json:"hostname"`
		ForceSSL bool   `json:"force_ssl"`
	}{h.APIID, h.ID, h.Name, h.Hostname, h.ForceSSL}

	return json.Marshal(temp)
}

// Validate validates the model.
func (h *Host) Validate(isInsert bool) aperrors.Errors {
	errors := make(aperrors.Errors)
	if h.Name == "" {
		errors.Add("name", "must not be blank")
	}
	if h.Hostname == "" {
		errors.Add("hostname", "must not be blank")
	}
	if h.CertContents() != "" || h.PrivateKeyContents() != "" {
		_, err := parsePem([]byte(h.PrivateKeyContents()))
		if err != nil {
			errors.Add("private_key", err.Error())
		}

		_, err = parsePem([]byte(h.CertContents()))
		if err != nil {
			errors.Add("cert", err.Error())
		}

		_, err = tls.X509KeyPair([]byte(h.CertContents()), []byte(h.PrivateKeyContents()))
		if err != nil {
			errors.Add("private_key", "invalid key pair")
			errors.Add("cert", "invalid key pair")
		}

	}
	return errors
}

// ValidateFromDatabaseError translates possible database constraint errors
// into validation errors.
func (h *Host) ValidateFromDatabaseError(err error) aperrors.Errors {
	errors := make(aperrors.Errors)
	if err.Error() == "UNIQUE constraint failed: hosts.api_id, hosts.name" ||
		err.Error() == `pq: duplicate key value violates unique constraint "hosts_api_id_name_key"` {
		errors.Add("name", "is already taken")
	}
	if err.Error() == "UNIQUE constraint failed: hosts.hostname" ||
		err.Error() == `pq: duplicate key value violates unique constraint "hosts_hostname_key"` {
		errors.Add("hostname", "is already taken")
	}
	return errors
}

// AllHostsForAPIIDAndAccountID returns all hosts on the Account's API in default order.
func AllHostsForAPIIDAndAccountID(db *apsql.DB, apiID, accountID int64) ([]*Host, error) {
	hosts := []*Host{}
	err := db.Select(&hosts, db.SQL("hosts/all"), apiID, accountID)
	return hosts, err
}

// AllHosts returns all hosts in an unspecified order.
func AllHosts(db *apsql.DB) ([]*Host, error) {
	hosts := []*Host{}
	err := db.Select(&hosts, db.SQL("hosts/all_routing"))
	return hosts, err
}

// AnyHostExists checks whether any hosts are set up.
func AnyHostExists(tx *apsql.Tx) (bool, error) {
	var count int64
	if err := tx.Get(&count, tx.SQL("hosts/count")); err != nil {
		return false, errors.New("Could not count hosts.")
	}

	return count > 0, nil
}

// FindHostForAPIIDAndAccountID returns the host with the id, api id, and account_id specified.
func FindHostForAPIIDAndAccountID(db *apsql.DB, id, apiID, accountID int64) (*Host, error) {
	host := Host{}
	err := db.Get(&host, db.SQL("hosts/find"), id, apiID, accountID)
	return &host, err
}

// DeleteHostForAPIIDAndAccountID deletes the host with the id, api_id and account_id specified.
func DeleteHostForAPIIDAndAccountID(tx *apsql.Tx, id, apiID, accountID, userID int64) error {
	err := tx.DeleteOne(tx.SQL("hosts/delete"), id, apiID, accountID)
	if err != nil {
		return err
	}
	err = tx.Notify("hosts", accountID, userID, apiID, 0, id, apsql.Delete)
	if err != nil {
		return err
	}
	// Notify regarding APIs since a host change can impact default base url of an API.
	return tx.Notify("apis", accountID, userID, apiID, 0, apiID, apsql.Update)
}

// FindHostForHostname returns the host with the hostname specified.
func FindHostForHostname(db *apsql.DB, hostname string) (*Host, error) {
	host := Host{}
	err := db.Get(&host, db.SQL("hosts/find_by_hostname"), hostname)
	return &host, err
}

// Insert inserts the host into the database as a new row.
func (h *Host) Insert(tx *apsql.Tx) (err error) {
	h.ID, err = tx.InsertOne(tx.SQL("hosts/insert"),
		h.APIID, h.AccountID, h.Name, h.Hostname, h.Cert, h.PrivateKey, h.ForceSSL)
	if err != nil {
		return err
	}
	err = tx.Notify("hosts", h.AccountID, h.UserID, h.APIID, 0, h.ID, apsql.Insert)
	if err != nil {
		return err
	}
	// Notify regarding APIs since a host change can impact default base url of an API.
	return tx.Notify("apis", h.AccountID, h.UserID, h.APIID, 0, h.APIID, apsql.Update)
}

// Update updates the host in the database.
func (h *Host) Update(tx *apsql.Tx) error {
	err := tx.UpdateOne(tx.SQL("hosts/update"),
		h.Name, h.Hostname, h.Cert, h.PrivateKey, h.ForceSSL, h.ID, h.APIID, h.AccountID)
	if err != nil {
		return err
	}
	err = tx.Notify("hosts", h.AccountID, h.UserID, h.APIID, 0, h.ID, apsql.Update)
	if err != nil {
		return err
	}
	// Notify regarding APIs since a host change can impact default base url of an API.
	return tx.Notify("apis", h.AccountID, h.UserID, h.APIID, 0, h.APIID, apsql.Update)
}
