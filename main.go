package main

import (
	"context"
	"encoding/json"
	"f0oster/adspy/activedirectory"
	"f0oster/adspy/activedirectory/ldaphelpers"
	"f0oster/adspy/config"
	"f0oster/adspy/database"
	"fmt"
)

func MarshalADObjectsToJSON(adObjects []activedirectory.ActiveDirectoryObject) ([]byte, error) {
	flat := make([]activedirectory.ADSnapshot, len(adObjects))
	for i, obj := range adObjects {
		flat[i] = activedirectory.ToADSnapshot(obj)
	}
	return json.MarshalIndent(flat, "", "  ")
}

func main() {

	ctx := context.Background()
	// database.ResetDatabase(ctx)

	adSpyConfig := config.LoadEnvConfig("settings.env")
	adInstance := activedirectory.NewActiveDirectoryInstance(adSpyConfig.BaseDN, adSpyConfig.DcFQDN, adSpyConfig.PageSize)
	adInstance.Connect(adSpyConfig.Username, adSpyConfig.Password)
	adInstance.LoadSchema()
	adInstance.FetchHighestUSN()

	db := database.NewDatabase("postgres://postgres:example@dockerprdap01:5432/adspy", "postgres://postgres:example@dockerprdap01:5432/postgres", ctx)
	db.Connect()
	db.InitalizeDomain(adInstance)

	ldapFilter := ldaphelpers.And(
		ldaphelpers.Eq("objectClass", "*"),
		ldaphelpers.Eq("objectCategory", "*"),
	).String()

	err := adInstance.FetchPagedEntriesWithCallback(ldapFilter, 1000, db.WriteObjectsConcurrent)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
