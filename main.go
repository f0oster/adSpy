package main

import (
	"f0oster/adspy/activedirectory"
	"f0oster/adspy/activedirectory/ldaphelpers"
	"f0oster/adspy/config"
	"f0oster/adspy/database"

	"context"
	"fmt"
	"log"
	"time"
)

func main() {

	adSpyConfig := config.LoadEnvConfig("settings.env")

	ctx := context.Background()
	database.ResetDatabase(ctx, adSpyConfig.ManagementDsn, adSpyConfig.AdSpyDsn)
	db := database.NewDatabase(adSpyConfig.AdSpyDsn, adSpyConfig.ManagementDsn, ctx)
	db.Connect()

	adInstance, err := activedirectory.NewActiveDirectoryInstance(adSpyConfig)

	if err != nil {
		log.Fatalf("failed to initialize Active Directory instance: %v", err)
	}

	err = db.InsertDomain(adInstance)

	if err != nil {
		log.Fatalf("failed to insert domain entry to database: %v", err)
	}

	for {
		ldapFilter := ldaphelpers.And(
			ldaphelpers.Eq("objectClass", "*"),
			ldaphelpers.Eq("objectCategory", "*"),
			ldaphelpers.Ge("uSNChanged", adInstance.HighestCommittedUSN),
		).String()
		err := adInstance.ForEachLDAPPage(ldapFilter, 1000, db.DispatchObjectWrites)
		adInstance.FetchHighestUSN()
		if err != nil {
			fmt.Printf("error: %v\n", err)
		}
		time.Sleep(1 * time.Second)
	}
}
